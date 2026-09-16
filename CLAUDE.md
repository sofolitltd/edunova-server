# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
go run .                              # Run server (reads .env, connects to DB, auto-migrates on startup)
go build -o edunova-server .         # Build binary
go test ./...                         # Run tests (no test files exist yet)
go vet ./...                          # Static analysis
```

There is no test suite, linter config, or CI in this repo currently. `go vet` and a successful `go build` are the closest thing to a pre-commit check.

This directory is not a git repository — if the user asks you to commit, ask them how they want it tracked first.

Required `.env` vars: `DATABASE_URL` (Neon PostgreSQL), `PORT` (default 8080), `JWT_SECRET`, plus optional `SMS_API_KEY`/`SMS_SENDER_ID`/`SMS_API_URL` (bulksmsbd.net) and `FCM_SERVER_KEY`/`FCM_SERVICE_ACCOUNT`/`FCM_PROJECT_ID` for push notifications. See `config/config.go` for defaults.

## Sibling repos

This Go server is the backend for two other apps, not present in this repo:

```
Flutter app:              /Users/reyad/Documents/Programming/Flutter/edunova
Next.js admin dashboard:  /Users/reyad/Documents/Programming/NextJs/edunova-admin
```

Changing a request/response shape or adding an endpoint here generally requires matching changes in the Flutter app's `lib/features/*/services/` and/or the Next.js admin's `src/lib/api.ts` — they are not auto-synced or code-generated from this repo.

## Architecture

Plain layered structure, no framework conventions beyond gin: `main.go` → `routes/routes.go` (all route wiring in one file) → `handlers/*.go` (one file per domain, business logic + SQL inline) → `database/db.go` (connection pool + migrations + demo-data seeding) and `models/*.go` (structs only, no ORM). `services/` holds outbound integrations (SMS, FCM, generic HTTP helper).

- **No ORM / query builder.** All SQL is hand-written with `pgx` (`database.DB`, a `*pgxpool.Pool`), directly inside handler functions. There is no repository layer — read the handler for a given endpoint to see its exact query.
- **Migrations are code, not files.** `database/db.go`'s `runMigrations()` runs a sequence of `runXMigration()` functions on every startup: `CREATE TABLE IF NOT EXISTS` for new tables, then idempotent `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` statements for evolving existing ones (e.g. the `courses` and `enrollments` tables have accumulated many ALTERs — check `runCourseMigration()` for the current full column set rather than assuming the original `CREATE TABLE`). There's no down-migration or version tracking; schema changes are additive and permanent.
- **Seed data.** `db.go` also seeds demo courses, hierarchy (class/subject/book/chapter/topic), expenses, batches, articles, notes, daily content, transitions, and the master admin — each guarded by a `COUNT(*) > 0` early-return so it only runs once. When adding a new domain that needs bootstrap data, follow this same idempotent-seed pattern.
- **Two independent, hand-rolled JWT schemes** — not a JWT library (no `golang-jwt` in `go.mod`). Both implement raw HMAC-SHA256 signing manually:
  - `middleware/jwt.go`: user tokens, keyed on `mobile`, signed with `JWT_SECRET`, 72h expiry.
  - `middleware/admin_jwt.go`: admin tokens, keyed on `email`, signed with `JWT_SECRET + "-admin"` (a different effective secret), 24h expiry.
  - `middleware/teacher_jwt.go`: teacher tokens, keyed on `email`, signed with `JWT_SECRET + "-teacher"`, 24h expiry — teachers are a fully independent identity (own `teachers` table, own login at `POST /api/admin/teacher-login`), not an `admin_users` role.
  - Correspondingly there are auth middlewares (`middleware/auth.go`'s `AuthRequired()` sets `user_id`/`mobile` in context; `middleware/admin.go`'s `AdminRequired()` sets `admin_id`/`admin_email`/`admin_role`; `TeacherRequired()` sets `teacher_id`/`teacher_email`) and two composed ones: `MasterRequired()` gates on `admin_role == "master_admin"` and must be chained *after* `AdminRequired()` (see the `master := secured.Group("")` block in `routes/routes.go`); `StaffRequired()` accepts *either* a valid admin token *or* a valid teacher token (tries admin first) so routes shared between the admin dashboard and the teacher portal — attendance, lessons, daily content, exams, results, doubts — are reachable by both (see the `staff := admin.Group("")` block).
- **Admin roles:** `master_admin` (full access, only role that can manage other admins) > `admin` > `moderator`. Role is stored per-admin in `admin_users.role` and read fresh from the DB on every `AdminRequired()` call (not embedded in the JWT), so role changes take effect immediately without requiring re-login. Teachers are **not** an admin role — see above.
- **Teacher writes on shared `staff` routes:** tables like `attendance.marked_by`, `daily_content.created_by`, `student_results.created_by` are nullable FKs to `admin_users(id)` only (no column for teacher attribution yet). Handlers for those (`AdminMarkAttendance`, `AdminCreateDailyContent`, `AdminCreateResult`) explicitly write SQL `NULL` when the actor is a teacher rather than an admin, to avoid FK violations or misattributing the row to an unrelated admin ID. `AdminResolveDoubt` and `AdminCreateLesson` get this for free since they pass the raw (possibly-unset) `admin_id` context value straight into the query.
- **Route registration:** everything is wired in one place, `routes/routes.go` — there's no per-feature router file. New endpoints get added to the relevant `api`/`auth`/`admin`/`secured`/`staff`/`master` group there, then implemented as a handler in the matching `handlers/*.go` file.
- **Models file split:** `models/models.go` has the original/auth-era types (`User`, `OTP`, course/exam basics); `models/admin.go` has essentially everything added since (course/exam/enrollment DTOs, the full question-bank hierarchy, finance, batches, notes, daily content, transitions). New domains should go in `models/admin.go`-style files rather than assuming `models/models.go` is authoritative.

## Question bank domain

Deepest/most complex feature — hierarchy is `Class → Subject → Book → Chapter → Topic`, each its own table with `id`, `name`, `name_bn`, `order_index`, referenced by nullable FKs on `questions`. Question types: `mcq, short_answer, very_short_answer, fill_blank, true_false, matching, descriptive, creative, problem_solving`; lifecycle status `draft → published → archived`. Bulk CSV import (both for the hierarchy and for questions) writes an audit row to `question_imports` and does two-pass duplicate detection (exact text match, then normalized — lowercased/depunctuated/whitespace-collapsed match) with row-level error reporting; there's no AI/semantic similarity check.

## Notifications

FCM push is triggered server-side on specific events (e.g. attendance marked → parent notified if student present) via `services/fcm.go`, using a service account JSON (`serviceAccountKey.json`, path configurable via `FCM_SERVICE_ACCOUNT`) rather than the legacy server-key API despite `FCM_SERVER_KEY` still being a config field. Device tokens are stored per-user in `device_tokens` and registered via `POST /api/device-token`.
