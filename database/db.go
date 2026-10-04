package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"edunova-server/config"
)

var DB *pgxpool.Pool

func Connect() {
	var err error
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	DB, err = pgxpool.New(ctx, config.AppConfig.DatabaseURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}

	if err = DB.Ping(ctx); err != nil {
		log.Fatalf("Unable to ping database: %v", err)
	}

	fmt.Println("Connected to Neon PostgreSQL")

	runMigrations()
}

func runMigrations() {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		full_name VARCHAR(255) NOT NULL,
		mobile VARCHAR(20) UNIQUE NOT NULL,
		password_hash VARCHAR(255) NOT NULL,
		verified BOOLEAN DEFAULT FALSE,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS otps (
		id SERIAL PRIMARY KEY,
		mobile VARCHAR(20) NOT NULL,
		code VARCHAR(6) NOT NULL,
		expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);
	`

	_, err := DB.Exec(context.Background(), query)
	if err != nil {
		log.Fatalf("Unable to run migrations: %v", err)
	}

	// Add verified column if it doesn't exist (for existing databases)
	_, _ = DB.Exec(context.Background(), `ALTER TABLE users ADD COLUMN IF NOT EXISTS verified BOOLEAN DEFAULT FALSE`)

	runAdminMigrations()
	runCourseMigration()
	runQuestionBankMigration()
	runClassBackfillMigration()
	runFinanceMigration()
	runSMSMigration()
	runBatchMigration()
	runAttendanceMigration()
	runDoubtTrackerMigration()
	runCalendarMigration()
	runLessonsMigration()
	runPaymentsMigration()
	runInvoicesMigration()
	runParentingHubMigration()
	runNotificationsHistoryMigration()
	runLiveExamMigration()
	runNotesMigration()
	runDailyContentMigration()
	runTransitionMigration()
	runStudentClassCleanupMigration()
	runResultsMigration()
	runPracticeMigration()
	runTeacherMigration()
	runBatchTeacherMigration()
	runBatchSubjectMigration()
	runOMRMigration()
	runOMRDesignMigration()
	runPromoCodeMigration()
	seedAdminUser()
	seedDemoHierarchy()
	seedDemoExpenses()
	seedDemoBatches()
	seedParentingHubArticles()
	seedDemoNotes()
	seedDemoDailyContent()
	seedDemoTransitions()
	seedDemoVocabulary()
	seedDemoSentenceExercises()

	fmt.Println("Migrations completed")
}

func runAdminMigrations() {
	query := `
	CREATE TABLE IF NOT EXISTS admin_users (
		id SERIAL PRIMARY KEY,
		email VARCHAR(255) UNIQUE NOT NULL,
		password_hash VARCHAR(255) NOT NULL,
		full_name VARCHAR(255) NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS courses (
		id SERIAL PRIMARY KEY,
		title VARCHAR(255) NOT NULL,
		description TEXT DEFAULT '',
		subject VARCHAR(255) NOT NULL,
		teacher VARCHAR(255) NOT NULL,
		schedule VARCHAR(255) DEFAULT '',
		color VARCHAR(20) DEFAULT '#6366F1',
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS exams (
		id SERIAL PRIMARY KEY,
		title VARCHAR(255) NOT NULL,
		course_id INT REFERENCES courses(id) ON DELETE CASCADE,
		date VARCHAR(50) NOT NULL,
		time VARCHAR(50) NOT NULL,
		duration VARCHAR(50) NOT NULL,
		total_questions INT DEFAULT 0,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS exam_questions (
		id SERIAL PRIMARY KEY,
		exam_id INT REFERENCES exams(id) ON DELETE CASCADE,
		question_text TEXT NOT NULL,
		option_a VARCHAR(500) NOT NULL,
		option_b VARCHAR(500) NOT NULL,
		option_c VARCHAR(500) NOT NULL,
		option_d VARCHAR(500) NOT NULL,
		correct_option INT NOT NULL CHECK (correct_option BETWEEN 1 AND 4),
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS exam_results (
		id SERIAL PRIMARY KEY,
		exam_id INT REFERENCES exams(id) ON DELETE CASCADE,
		user_id INT REFERENCES users(id) ON DELETE CASCADE,
		score INT NOT NULL DEFAULT 0,
		total_questions INT NOT NULL DEFAULT 0,
		completed_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS enrollments (
		id SERIAL PRIMARY KEY,
		course_id INT REFERENCES courses(id) ON DELETE SET NULL,
		full_name VARCHAR(255) NOT NULL,
		mobile VARCHAR(20) NOT NULL,
		payment_method VARCHAR(50) NOT NULL DEFAULT 'manual',
		mobile_banking VARCHAR(50) DEFAULT '',
		amount INT NOT NULL DEFAULT 0,
		sent_from VARCHAR(20) DEFAULT '',
		sent_to VARCHAR(20) DEFAULT '',
		referral_source VARCHAR(100) DEFAULT '',
		status VARCHAR(20) NOT NULL DEFAULT 'pending',
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);
	`

	_, err := DB.Exec(context.Background(), query)
	if err != nil {
		log.Fatalf("Unable to run admin migrations: %v", err)
	}
}

func runCourseMigration() {
	alters := []string{
		`ALTER TABLE courses ADD COLUMN IF NOT EXISTS title_bn VARCHAR(255) DEFAULT ''`,
		`ALTER TABLE courses ADD COLUMN IF NOT EXISTS price INT DEFAULT 0`,
		`ALTER TABLE courses ADD COLUMN IF NOT EXISTS old_price INT DEFAULT 0`,
		`ALTER TABLE courses ADD COLUMN IF NOT EXISTS duration VARCHAR(50) DEFAULT ''`,
		`ALTER TABLE courses ADD COLUMN IF NOT EXISTS badge VARCHAR(50) DEFAULT ''`,
		`ALTER TABLE courses ADD COLUMN IF NOT EXISTS students_count INT DEFAULT 0`,
		`ALTER TABLE courses ADD COLUMN IF NOT EXISTS classes_count INT DEFAULT 0`,
		`ALTER TABLE courses ADD COLUMN IF NOT EXISTS exams_count INT DEFAULT 0`,
		`ALTER TABLE courses ADD COLUMN IF NOT EXISTS rating DECIMAL(2,1) DEFAULT 0.0`,
		`ALTER TABLE courses ADD COLUMN IF NOT EXISTS reviews_count INT DEFAULT 0`,
		`ALTER TABLE courses ADD COLUMN IF NOT EXISTS gradient VARCHAR(100) DEFAULT 'from-primary to-primary-dark'`,
		`ALTER TABLE courses ADD COLUMN IF NOT EXISTS curriculum JSONB DEFAULT '[]'`,
		`ALTER TABLE courses ADD COLUMN IF NOT EXISTS features JSONB DEFAULT '[]'`,
		`ALTER TABLE courses ADD COLUMN IF NOT EXISTS class_level VARCHAR(20) DEFAULT ''`,
		`ALTER TABLE courses ADD COLUMN IF NOT EXISTS instructors TEXT DEFAULT ''`,
		`ALTER TABLE courses ADD COLUMN IF NOT EXISTS type VARCHAR(20) NOT NULL DEFAULT 'online'`,
		`ALTER TABLE enrollments ADD COLUMN IF NOT EXISTS user_id INT REFERENCES users(id) ON DELETE SET NULL`,
		`ALTER TABLE enrollments ADD COLUMN IF NOT EXISTS enrolled_by VARCHAR(20) NOT NULL DEFAULT 'self'`,
		`ALTER TABLE enrollments ADD COLUMN IF NOT EXISTS batch_id INT REFERENCES batches(id) ON DELETE SET NULL`,
		`ALTER TABLE enrollments ADD COLUMN IF NOT EXISTS student_id VARCHAR(30) NOT NULL DEFAULT ''`,
		`ALTER TABLE enrollments ADD COLUMN IF NOT EXISTS transaction_id VARCHAR(100) DEFAULT ''`,
		`ALTER TABLE enrollments ADD COLUMN IF NOT EXISTS fee_breakdown JSONB`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_enrollments_student_id ON enrollments (student_id) WHERE student_id <> ''`,
	}

	for _, q := range alters {
		_, _ = DB.Exec(context.Background(), q)
	}

	fmt.Println("Course migration completed")
}

func runQuestionBankMigration() {
	// Hierarchy tables
	hierarchy := `
	CREATE TABLE IF NOT EXISTS classes (
		id SERIAL PRIMARY KEY,
		name VARCHAR(100) NOT NULL,
		name_bn VARCHAR(100) DEFAULT '',
		order_index INT DEFAULT 0,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS subjects (
		id SERIAL PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		name_bn VARCHAR(255) DEFAULT '',
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS books (
		id SERIAL PRIMARY KEY,
		subject_id INT REFERENCES subjects(id) ON DELETE CASCADE,
		class_id INT REFERENCES classes(id) ON DELETE CASCADE,
		name VARCHAR(255) NOT NULL,
		name_bn VARCHAR(255) DEFAULT '',
		publisher VARCHAR(255) DEFAULT '',
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS chapters (
		id SERIAL PRIMARY KEY,
		book_id INT REFERENCES books(id) ON DELETE CASCADE,
		name VARCHAR(255) NOT NULL,
		name_bn VARCHAR(255) DEFAULT '',
		order_index INT DEFAULT 0,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS topics (
		id SERIAL PRIMARY KEY,
		chapter_id INT REFERENCES chapters(id) ON DELETE CASCADE,
		name VARCHAR(255) NOT NULL,
		name_bn VARCHAR(255) DEFAULT '',
		order_index INT DEFAULT 0,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);
	`
	_, err := DB.Exec(context.Background(), hierarchy)
	if err != nil {
		log.Fatalf("Unable to run question bank hierarchy migrations: %v", err)
	}

	// Book editions: a new year's book is a clone of the old one (see
	// AdminCloneBook); the old edition is archived, never edited in place.
	for _, q := range []string{
		`ALTER TABLE books ADD COLUMN IF NOT EXISTS academic_year INT NOT NULL DEFAULT 0`,
		`ALTER TABLE books ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT TRUE`,
		`ALTER TABLE books ADD COLUMN IF NOT EXISTS replaces_book_id INT REFERENCES books(id) ON DELETE SET NULL`,
	} {
		if _, err := DB.Exec(context.Background(), q); err != nil {
			log.Printf("Warning: failed to alter books table: %v", err)
		}
	}

	_, err = DB.Exec(context.Background(), `ALTER TABLE classes ADD COLUMN IF NOT EXISTS code VARCHAR(2) DEFAULT ''`)
	if err != nil {
		log.Printf("Warning: failed to add classes.code column: %v", err)
	}

	// Question bank table
	questions := `
	CREATE TABLE IF NOT EXISTS questions (
		id SERIAL PRIMARY KEY,
		class_id INT REFERENCES classes(id) ON DELETE SET NULL,
		subject_id INT REFERENCES subjects(id) ON DELETE SET NULL,
		book_id INT REFERENCES books(id) ON DELETE SET NULL,
		chapter_id INT REFERENCES chapters(id) ON DELETE SET NULL,
		topic_id INT REFERENCES topics(id) ON DELETE SET NULL,
		question_type VARCHAR(50) NOT NULL DEFAULT 'mcq',
		question_text TEXT NOT NULL,
		options JSONB DEFAULT NULL,
		answer TEXT NOT NULL,
		explanation TEXT DEFAULT '',
		marks INT DEFAULT 1,
		difficulty VARCHAR(20) DEFAULT 'medium',
		tags TEXT[] DEFAULT '{}',
		source VARCHAR(255) DEFAULT '',
		source_page INT DEFAULT NULL,
		language VARCHAR(10) DEFAULT 'bn',
		status VARCHAR(20) DEFAULT 'draft',
		created_by INT REFERENCES admin_users(id) ON DELETE SET NULL,
		reviewed_by INT REFERENCES admin_users(id) ON DELETE SET NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		version INT DEFAULT 1,
		usage_count INT DEFAULT 0,
		last_used_at TIMESTAMP WITH TIME ZONE DEFAULT NULL
	);
	`
	_, err = DB.Exec(context.Background(), questions)
	if err != nil {
		log.Fatalf("Unable to run questions migration: %v", err)
	}

	// Bulk import tracking table
	imports := `
	CREATE TABLE IF NOT EXISTS question_imports (
		id SERIAL PRIMARY KEY,
		filename VARCHAR(255) NOT NULL,
		total_rows INT DEFAULT 0,
		imported INT DEFAULT 0,
		duplicates INT DEFAULT 0,
		errors INT DEFAULT 0,
		error_details JSONB DEFAULT '[]',
		status VARCHAR(20) DEFAULT 'pending',
		created_by INT REFERENCES admin_users(id) ON DELETE SET NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		completed_at TIMESTAMP WITH TIME ZONE DEFAULT NULL
	);
	`
	_, err = DB.Exec(context.Background(), imports)
	if err != nil {
		log.Fatalf("Unable to run question imports migration: %v", err)
	}

	// Indexes for performance
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_questions_class ON questions(class_id)`,
		`CREATE INDEX IF NOT EXISTS idx_questions_subject ON questions(subject_id)`,
		`CREATE INDEX IF NOT EXISTS idx_questions_book ON questions(book_id)`,
		`CREATE INDEX IF NOT EXISTS idx_questions_chapter ON questions(chapter_id)`,
		`CREATE INDEX IF NOT EXISTS idx_questions_topic ON questions(topic_id)`,
		`CREATE INDEX IF NOT EXISTS idx_questions_type ON questions(question_type)`,
		`CREATE INDEX IF NOT EXISTS idx_questions_difficulty ON questions(difficulty)`,
		`CREATE INDEX IF NOT EXISTS idx_questions_status ON questions(status)`,
		`CREATE INDEX IF NOT EXISTS idx_questions_created_by ON questions(created_by)`,
		`CREATE INDEX IF NOT EXISTS idx_books_subject ON books(subject_id)`,
		`CREATE INDEX IF NOT EXISTS idx_books_class ON books(class_id)`,
		`CREATE INDEX IF NOT EXISTS idx_chapters_book ON chapters(book_id)`,
		`CREATE INDEX IF NOT EXISTS idx_topics_chapter ON topics(chapter_id)`,
		`CREATE INDEX IF NOT EXISTS idx_question_imports_status ON question_imports(status)`,
	}
	for _, idx := range indexes {
		_, _ = DB.Exec(context.Background(), idx)
	}

	fmt.Println("Question bank migration completed")
}

// runClassBackfillMigration ensures classes 3-8 all exist in the hierarchy's
// `classes` table, which is now the single canonical source for every class
// picklist in the admin panel (Notes, Daily Content, Vocabulary, Sentence
// Exercises, Courses, Batches) — replacing what used to be a hardcoded
// ["3".."8"] array duplicated across each of those pages. It only fills
// gaps (matched by order_index, which already held the bare digit for the
// classes an admin had entered by hand) so it never touches or duplicates
// existing entries.
func runClassBackfillMigration() {
	ctx := context.Background()
	type classSeed struct {
		order        int
		name, nameBn string
	}
	classes := []classSeed{
		{3, "Class 3", "৩য় শ্রেণি"},
		{4, "Class 4", "৪র্থ শ্রেণি"},
		{5, "Class 5", "৫ম শ্রেণি"},
		{6, "Class 6", "৬ষ্ঠ শ্রেণি"},
		{7, "Class 7", "৭ম শ্রেণি"},
		{8, "Class 8", "৮ম শ্রেণি"},
	}
	for _, cl := range classes {
		var exists int
		_ = DB.QueryRow(ctx, `SELECT COUNT(*) FROM classes WHERE order_index = $1`, cl.order).Scan(&exists)
		if exists > 0 {
			continue
		}
		_, err := DB.Exec(ctx,
			`INSERT INTO classes (name, name_bn, order_index) VALUES ($1, $2, $3)`,
			cl.name, cl.nameBn, cl.order)
		if err != nil {
			log.Printf("Warning: failed to backfill class %q: %v", cl.name, err)
		}
	}

	// Backfill a 2-digit code for numbered classes so batch codes can be
	// generated for them; never overwrites a code an admin has already set.
	_, err := DB.Exec(ctx,
		`UPDATE classes SET code = lpad(order_index::text, 2, '0')
		 WHERE code = '' AND order_index BETWEEN 1 AND 99`)
	if err != nil {
		log.Printf("Warning: failed to backfill class codes: %v", err)
	}
}

func runTeacherMigration() {
	_, err := DB.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS teachers (
		id SERIAL PRIMARY KEY,
		email VARCHAR(255) UNIQUE NOT NULL,
		password_hash VARCHAR(255) NOT NULL,
		full_name VARCHAR(255) NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`)
	if err != nil {
		log.Printf("Warning: failed to create teachers table: %v", err)
	}

	alters := []string{
		`ALTER TABLE teachers ADD COLUMN IF NOT EXISTS phone VARCHAR(20) DEFAULT ''`,
		`ALTER TABLE teachers ADD COLUMN IF NOT EXISTS education TEXT DEFAULT ''`,
		`ALTER TABLE teachers ADD COLUMN IF NOT EXISTS bio TEXT DEFAULT ''`,
		`ALTER TABLE teachers ADD COLUMN IF NOT EXISTS address TEXT DEFAULT ''`,
		`ALTER TABLE teachers ADD COLUMN IF NOT EXISTS photo_url TEXT DEFAULT ''`,
		`ALTER TABLE teachers ADD COLUMN IF NOT EXISTS join_date DATE`,
		`ALTER TABLE teachers ADD COLUMN IF NOT EXISTS leave_date DATE`,
		`ALTER TABLE teachers ADD COLUMN IF NOT EXISTS nickname VARCHAR(100) DEFAULT ''`,
		`ALTER TABLE teachers ADD COLUMN IF NOT EXISTS gender VARCHAR(10) DEFAULT ''`,
	}
	for _, q := range alters {
		if _, err := DB.Exec(context.Background(), q); err != nil {
			log.Printf("Warning: failed to alter teachers table: %v", err)
		}
	}
}

func runBatchTeacherMigration() {
	_, err := DB.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS batch_teachers (
		id SERIAL PRIMARY KEY,
		batch_id INT REFERENCES batches(id) ON DELETE CASCADE,
		teacher_id INT REFERENCES teachers(id) ON DELETE CASCADE,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		UNIQUE(batch_id, teacher_id)
	)`)
	if err != nil {
		log.Printf("Warning: failed to create batch_teachers table: %v", err)
	}
}

func runBatchSubjectMigration() {
	// No UNIQUE(batch_id, subject_id) here: a subject can be taught at
	// several different times in the same batch (e.g. two periods a week),
	// so each schedule slot is its own row.
	_, err := DB.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS batch_subjects (
		id SERIAL PRIMARY KEY,
		batch_id INT REFERENCES batches(id) ON DELETE CASCADE,
		subject_id INT REFERENCES subjects(id) ON DELETE CASCADE,
		days TEXT[] DEFAULT '{}',
		start_time VARCHAR(10) DEFAULT '',
		end_time VARCHAR(10) DEFAULT '',
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`)
	if err != nil {
		log.Printf("Warning: failed to create batch_subjects table: %v", err)
	}
	_, err = DB.Exec(context.Background(), `
	ALTER TABLE batch_subjects ADD COLUMN IF NOT EXISTS teacher_id INT REFERENCES teachers(id) ON DELETE SET NULL`)
	if err != nil {
		log.Printf("Warning: failed to add teacher_id to batch_subjects: %v", err)
	}
	// Older deployments created the table with this constraint before
	// multiple time slots per subject were supported; drop it if present.
	_, err = DB.Exec(context.Background(), `
	ALTER TABLE batch_subjects DROP CONSTRAINT IF EXISTS batch_subjects_batch_id_subject_id_key`)
	if err != nil {
		log.Printf("Warning: failed to drop batch_subjects unique constraint: %v", err)
	}

	// Who taught a batch's subject, and when. teacher_name is a snapshot, so
	// renaming or deleting a teacher later never rewrites past history.
	_, err = DB.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS batch_subject_teacher_history (
		id SERIAL PRIMARY KEY,
		batch_id INT NOT NULL REFERENCES batches(id) ON DELETE CASCADE,
		subject_id INT NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
		batch_subject_id INT REFERENCES batch_subjects(id) ON DELETE SET NULL,
		teacher_id INT REFERENCES teachers(id) ON DELETE SET NULL,
		teacher_name VARCHAR(255) NOT NULL DEFAULT '',
		from_date DATE NOT NULL,
		to_date DATE,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`)
	if err != nil {
		log.Printf("Warning: failed to create batch_subject_teacher_history table: %v", err)
	}
	_, _ = DB.Exec(context.Background(), `CREATE INDEX IF NOT EXISTS idx_bst_history_batch ON batch_subject_teacher_history(batch_id)`)
	_, err = DB.Exec(context.Background(), `
	INSERT INTO batch_subject_teacher_history (batch_id, subject_id, batch_subject_id, teacher_id, teacher_name, from_date)
	SELECT bs.batch_id, bs.subject_id, bs.id, bs.teacher_id, t.full_name, COALESCE(bs.created_at::date, CURRENT_DATE)
	FROM batch_subjects bs
	JOIN teachers t ON t.id = bs.teacher_id
	WHERE NOT EXISTS (SELECT 1 FROM batch_subject_teacher_history h WHERE h.batch_subject_id = bs.id)`)
	if err != nil {
		log.Printf("Warning: failed to backfill batch_subject_teacher_history: %v", err)
	}
}

func seedAdminUser() {
	// Add role column if not exists
	_, _ = DB.Exec(context.Background(), `ALTER TABLE admin_users ADD COLUMN IF NOT EXISTS role VARCHAR(50) DEFAULT 'admin'`)

	// Delete old default admin if exists
	_, _ = DB.Exec(context.Background(), `DELETE FROM admin_users WHERE email = $1`, "admin@edunova.com")

	// Always ensure master admin exists
	var masterCount int
	_ = DB.QueryRow(context.Background(), `SELECT COUNT(*) FROM admin_users WHERE email = $1`, "asifreyad1@gmail.com").Scan(&masterCount)
	if masterCount == 0 {
		hash, err := bcrypt.GenerateFromPassword([]byte("12345678"), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("Failed to hash master admin password: %v", err)
		} else {
			_, err = DB.Exec(
				context.Background(),
				`INSERT INTO admin_users (email, password_hash, full_name, role) VALUES ($1, $2, $3, $4)
				 ON CONFLICT (email) DO NOTHING`,
				"asifreyad1@gmail.com", string(hash), "Asif Reyad", "master_admin",
			)
			if err != nil {
				log.Printf("Failed to seed master admin: %v", err)
			} else {
				fmt.Println("Seeded master admin: asifreyad1@gmail.com / 12345678")
			}
		}
	}
}

func Close() {
	DB.Close()
}

func seedDemoCourses() {
	var count int
	_ = DB.QueryRow(context.Background(), `SELECT COUNT(*) FROM courses`).Scan(&count)
	if count > 0 {
		return
	}

	type course struct {
		title         string
		titleBn       string
		description   string
		subject       string
		teacher       string
		instructors   string
		classLevel    string
		courseType    string
		schedule      string
		color         string
		gradient      string
		price         int
		oldPrice      int
		duration      string
		badge         string
		studentsCount int
		classesCount  int
		examsCount    int
		rating        float64
		reviewsCount  int
		curriculum    string
		features      string
	}

	courses := []course{
		{
			title:         "Class 8 Academic Program 2026",
			titleBn:       "একাডেমিক প্রোগ্রাম ২০২৬ (৮ম শ্রেণী)",
			description:   "Comprehensive academic program for Class 8 students covering all subjects. Build strong foundations for future board exams with interactive live classes.",
			subject:       "All Subjects",
			teacher:       "Md Towfiqure Rehman",
			instructors:   "Md Towfiqure Rehman, Ayesha Tabassum",
			classLevel:    "8",
			courseType:    "offline",
			schedule:      "Sat-Tue, 5:00 PM - 7:00 PM",
			color:         "#F59E0B",
			gradient:      "from-warning to-amber-600",
			price:         990,
			oldPrice:      1490,
			duration:      "3 Months",
			badge:         "Best Value",
			studentsCount: 520,
			classesCount:  200,
			examsCount:    80,
			rating:        4.9,
			reviewsCount:  1100,
			curriculum:    `[{"month":"Month 1 — Basics","topics":["Math: Number System","English: Grammar","Bengali: prose","Science: Introduction","Social Science: Basics"]},{"month":"Month 2 — Intermediate","topics":["Math: Algebra","English: Writing","Bengali: Poetry","Science: Life Science","Social Science: Geography"]},{"month":"Month 3 — Final Prep","topics":["Mock Exams","Revision","Exam Strategy","Previous Year Papers","Tips"]}]`,
			features:      `["200+ Live Classes","80+ Practice Exams","Study Materials","Doubt Sessions","Parent Reports","Certificate"]`,
		},
		{
			title:         "Class 7 Academic Program 2026",
			titleBn:       "একাডেমিক প্রোগ্রাম ২০২৬ (৭ম শ্রেণী)",
			description:   "Well-structured academic program for Class 7 students. Strengthen core concepts in Math, Science, English, and Bengali.",
			subject:       "All Subjects",
			teacher:       "Ayesha Tabassum",
			instructors:   "Ayesha Tabassum, Fatema Khatun",
			classLevel:    "7",
			courseType:    "offline",
			schedule:      "Sun-Wed, 5:00 PM - 6:30 PM",
			color:         "#3B82F6",
			gradient:      "from-blue-500 to-blue-600",
			price:         990,
			oldPrice:      1290,
			duration:      "3 Months",
			badge:         "",
			studentsCount: 410,
			classesCount:  180,
			examsCount:    70,
			rating:        4.7,
			reviewsCount:  780,
			curriculum:    `[{"month":"Month 1 — Basics","topics":["Math: Fractions & Decimals","English: Reading","Bengali: Prose","Science: Environment","Social Science: History"]},{"month":"Month 2 — Intermediate","topics":["Math: Geometry","English: Writing","Bengali: Poetry","Science: Matter","Social Science: Civics"]},{"month":"Month 3 — Final Prep","topics":["Mock Exams","Revision","Exam Strategy","Practice Tests","Tips"]}]`,
			features:      `["180+ Live Classes","70+ Practice Exams","Study Materials","Doubt Sessions","Progress Tracking","Certificate"]`,
		},
		{
			title:         "Primary Scholarship Foundation Batch",
			titleBn:       "প্রাথমিক বৃত্তি Foundation ব্যাচ",
			description:   "Foundation course for primary scholarship preparation. Develop strong basics in Math, English, Bengali, and General Knowledge for scholarship exams.",
			subject:       "Scholarship Prep",
			teacher:       "Asifuzzaman Reyad",
			instructors:   "Asifuzzaman Reyad, Ayesha Tabassum",
			classLevel:    "5",
			courseType:    "offline",
			schedule:      "Fri-Sat, 10:00 AM - 12:00 PM",
			color:         "#8B5CF6",
			gradient:      "from-purple-500 to-purple-600",
			price:         499,
			oldPrice:      799,
			duration:      "2 Months",
			badge:         "Hot",
			studentsCount: 680,
			classesCount:  180,
			examsCount:    100,
			rating:        4.9,
			reviewsCount:  1500,
			curriculum:    `[{"month":"Month 1 — Foundation","topics":["Math: Mental Math","English: Vocabulary","Bengali: Grammar","General Knowledge: Bangladesh","Logic: Patterns"]},{"month":"Month 2 — Final Prep","topics":["Mock Scholarship Exams","Time Management","Previous Year Papers","Speed Tests","Exam Strategy"]}]`,
			features:      `["180+ Live Classes","100+ Practice Exams","Scholarship Materials","Speed Training","Parent Reports","Certificate"]`,
		},
		{
			title:         "Free Model Test — Class 8",
			titleBn:       "ফ্রী মডেল টেস্ট — ৮ম শ্রেণী",
			description:   "Free model test for Class 8 students. Practice with real exam-style questions and check your preparation level.",
			subject:       "All Subjects",
			teacher:       "Md Towfiqure Rehman",
			instructors:   "Md Towfiqure Rehman",
			classLevel:    "8",
			courseType:    "free",
			schedule:      "Anytime",
			color:         "#10B981",
			gradient:      "from-emerald-500 to-emerald-600",
			price:         0,
			oldPrice:      0,
			duration:      "",
			badge:         "Free",
			studentsCount: 1200,
			classesCount:  0,
			examsCount:    5,
			rating:        4.5,
			reviewsCount:  300,
			curriculum:    `[]`,
			features:      `["5 Model Tests","Instant Results","No Enrollment Needed"]`,
		},
	}

	for _, c := range courses {
		_, err := DB.Exec(
			context.Background(),
			`INSERT INTO courses (title, title_bn, description, subject, teacher, instructors, class_level, type, schedule, color, gradient,
				price, old_price, duration, badge, students_count, classes_count, exams_count,
				rating, reviews_count, curriculum, features)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21::jsonb,$22::jsonb)`,
			c.title, c.titleBn, c.description, c.subject, c.teacher, c.instructors, c.classLevel, c.courseType, c.schedule,
			c.color, c.gradient, c.price, c.oldPrice, c.duration, c.badge,
			c.studentsCount, c.classesCount, c.examsCount, c.rating, c.reviewsCount,
			c.curriculum, c.features,
		)
		if err != nil {
			log.Printf("Failed to seed course %q: %v", c.title, err)
		}
	}

	fmt.Printf("Seeded %d demo courses\n", len(courses))
}

func seedClassLevelCourses() {
	ctx := context.Background()

	// Update existing courses with class levels based on title
	_, _ = DB.Exec(ctx, `UPDATE courses SET class_level='10' WHERE title ILIKE '%SSC 2027%'`)
	_, _ = DB.Exec(ctx, `UPDATE courses SET class_level='9' WHERE title ILIKE '%SSC 2028%' OR title ILIKE '%Class 9%'`)
	_, _ = DB.Exec(ctx, `UPDATE courses SET class_level='8' WHERE title ILIKE '%Class 8%'`)
	_, _ = DB.Exec(ctx, `UPDATE courses SET class_level='7' WHERE title ILIKE '%Class 7%'`)
	_, _ = DB.Exec(ctx, `UPDATE courses SET class_level='3-5' WHERE title ILIKE '%Primary%' OR title ILIKE '%Scholarship%'`)

	// Only add new courses if we don't already have class 3-6 courses
	var count int
	_ = DB.QueryRow(ctx, `SELECT COUNT(*) FROM courses WHERE class_level IN ('3','4','5','6')`).Scan(&count)
	if count > 0 {
		return
	}

	type course struct {
		title, titleBn, description, subject, teacher, instructors, classLevel, courseType string
		schedule, color, gradient, duration, badge                                         string
		price, oldPrice, studentsCount, classesCount, examsCount                           int
		rating                                                                             float64
		reviewsCount                                                                       int
		curriculum, features                                                               string
	}

	courses := []course{
		{
			title:       "Class 3 Academic Program 2026",
			titleBn:     "তৃতীয় শ্রেণি একাডেমিক প্রোগ্রাম ২০২৬",
			description: "Comprehensive academic program for Class 3 students covering Mathematics, English, Bangla, and Science. Fun interactive classes with experienced teachers to build strong foundational skills.",
			subject:     "All Subjects",
			teacher:     "Ayesha Tabassum",
			instructors: "Ayesha Tabassum, Fatema Khatun",
			classLevel:  "3",
			courseType:  "offline",
			schedule:    "Sat-Tue, 4:00 PM - 5:30 PM",
			color:       "#10B981",
			gradient:    "from-emerald-500 to-emerald-700",
			price:       499, oldPrice: 799, duration: "3 Months", badge: "New",
			studentsCount: 120, classesCount: 48, examsCount: 24, rating: 4.7, reviewsCount: 85,
			curriculum: `[{"month":"Month 1 — Basics","topics":["Math: Numbers & Place Value","English: Alphabet & Phonics","Bangla: বর্ণ ও শব্দ","Science: Our Body"]},{"month":"Month 2 — Building","topics":["Math: Addition & Subtraction","English: Simple Sentences","Bangla: বাক্য গঠন","Science: Animals & Plants"]},{"month":"Month 3 — Advancing","topics":["Math: Multiplication Basics","English: Reading Comprehension","Bangla: গল্প ও কবিতা","Science: Weather & Seasons"]}]`,
			features:   `["48 Live Classes","24 Practice Exams","Interactive Sessions","Study Materials","Progress Reports","Certificate"]`,
		},
		{
			title:       "Class 4 Academic Program 2026",
			titleBn:     "চতুর্থ শ্রেণি একাডেমিক প্রোগ্রাম ২০২৬",
			description: "Structured academic program for Class 4 students. Focuses on building core skills in Math, English, Bangla, and Science with engaging lessons and regular assessments.",
			subject:     "All Subjects",
			teacher:     "Fatema Khatun",
			instructors: "Fatema Khatun, Md Towfiqure Rehman",
			classLevel:  "4",
			courseType:  "offline",
			schedule:    "Sat-Tue, 4:30 PM - 6:00 PM",
			color:       "#F59E0B",
			gradient:    "from-amber-500 to-amber-700",
			price:       499, oldPrice: 799, duration: "3 Months", badge: "Popular",
			studentsCount: 145, classesCount: 48, examsCount: 24, rating: 4.8, reviewsCount: 102,
			curriculum: `[{"month":"Month 1 — Foundation","topics":["Math: Large Numbers & Fractions","English: Grammar Fundamentals","Bangla: সমার্থক ও বিপরীতার্থক","Science: Food & Nutrition"]},{"month":"Month 2 — Growth","topics":["Math: Geometry Basics","English: Writing Skills","Bangla: ছোটগল্প","Science: Materials & Their Properties"]},{"month":"Month 3 — Mastery","topics":["Math: Data & Measurement","English: Poetry & Prose","Bangla: নাটক ও প্রবন্ধ","Science: Forces & Motion"]}]`,
			features:   `["48 Live Classes","24 Practice Exams","Interactive Sessions","Study Materials","Parent Meetings","Certificate"]`,
		},
		{
			title:       "Class 5 Scholarship Preparation 2026",
			titleBn:     "পঞ্চম শ্রেণি বৃত্তি প্রস্তুতি ২০২৬",
			description: "Intensive scholarship preparation course for Class 5 students. Covers Math, English, Bangla, and Science with focus on competitive exam strategies and problem-solving techniques.",
			subject:     "Scholarship Prep",
			teacher:     "Asifuzzaman Reyad",
			instructors: "Asifuzzaman Reyad, Ayesha Tabassum, Md Towfiqure Rehman",
			classLevel:  "5",
			courseType:  "offline",
			schedule:    "Sat-Wed, 5:00 PM - 7:00 PM",
			color:       "#EF4444",
			gradient:    "from-red-500 to-red-700",
			price:       799, oldPrice: 1299, duration: "4 Months", badge: "Hot",
			studentsCount: 280, classesCount: 80, examsCount: 40, rating: 4.9, reviewsCount: 312,
			curriculum: `[{"month":"Month 1 — Core","topics":["Math: Number Theory","English: Advanced Grammar","Bangla: ব্যাকরণ","Science: General Knowledge"]},{"month":"Month 2 — Problem Solving","topics":["Math: Olympiad Problems","English: Comprehension","Bangla: অনুচ্ছেদ রচনা","Science: Logic & Reasoning"]},{"month":"Month 3 — Advanced","topics":["Math: Geometry & Algebra","English: Essay Writing","Bangla: সাহিত্য","Science: Environmental Science"]},{"month":"Month 4 — Final Prep","topics":["Mock Scholarship Exams","Time Management","Revision Sessions","Exam Strategy"]}]`,
			features:   `["80 Live Classes","40 Mock Exams","Scholarship Strategies","3 Expert Teachers","Performance Analytics","Study Materials PDF"]`,
		},
		{
			title:       "Class 6 Foundation Batch 2026",
			titleBn:     "ষষ্ঠ শ্রেণি ফাউন্ডেশন ব্যাচ ২০২৬",
			description: "Strong foundation program for Class 6 students transitioning to secondary education. Covers all major subjects with emphasis on building analytical and critical thinking skills.",
			subject:     "All Subjects",
			teacher:     "Md Towfiqure Rehman",
			instructors: "Md Towfiqure Rehman, Fatema Khatun",
			classLevel:  "6",
			courseType:  "offline",
			schedule:    "Sat-Mon, 6:00 PM - 8:00 PM",
			color:       "#8B5CF6",
			gradient:    "from-violet-500 to-violet-700",
			price:       699, oldPrice: 999, duration: "3 Months", badge: "Best Value",
			studentsCount: 190, classesCount: 60, examsCount: 30, rating: 4.7, reviewsCount: 156,
			curriculum: `[{"month":"Month 1 — Transition","topics":["Math: Integers & Fractions","English: Tenses & Voice","Bangla: ভাষা ও সাহিত্য","Science: Cell & Tissues"]},{"month":"Month 2 — Integration","topics":["Math: Algebra Basics","English: Letter & Report Writing","Bangla: প্রবন্ধ রচনা","Science: Matter & Energy"]},{"month":"Month 3 — Expansion","topics":["Math: Ratio & Percentage","English: Translation Skills","Bangla: প্রতিবাদী সাহিত্য","Science: Earth & Universe"]}]`,
			features:   `["60 Live Classes","30 Practice Exams","2 Expert Teachers","Study Materials","Doubt Clearing","Parent Updates"]`,
		},
		{
			title:       "Class 3 Bangla Medium 2026",
			titleBn:     "তৃতীয় শ্রেণি বাংলা মাধ্যম ২০২৬",
			description: "Specialized Bangla medium program for Class 3 students. All classes conducted in Bangla with focus on Bangla literature, mathematics, and general knowledge.",
			subject:     "All Subjects",
			teacher:     "Fatema Khatun",
			instructors: "Fatema Khatun",
			classLevel:  "3",
			courseType:  "offline",
			schedule:    "Sun-Wed, 3:30 PM - 5:00 PM",
			color:       "#06B6D4",
			gradient:    "from-cyan-500 to-cyan-700",
			price:       399, oldPrice: 599, duration: "3 Months", badge: "",
			studentsCount: 85, classesCount: 48, examsCount: 20, rating: 4.6, reviewsCount: 52,
			curriculum: `[{"month":"Month 1 — বীজগণিত","topics":["সংখ্যা পদ্ধতি","যোগ ও বিয়োগ","বাংলা বর্ণমালা","প্রকৃতি পরিচিতি"]},{"month":"Month 2 — ভাষা","topics":["গুণন ও ভাগ","সরল বাক্য","ছোটগল্প পাঠ","প্রাণী পরিচিতি"]},{"month":"Month 3 — সাধারণ জ্ঞান","topics":["ভগ্নাংশ","অনুচ্ছেদ পাঠ","কবিতা","আমাদের পরিবেশ"]}]`,
			features:   `["বাংলা মাধ্যমে পাঠদান","৪৮টি লাইভ ক্লাস","২০টি পরীক্ষা","স্টাডি ম্যাটেরিয়াল","অগ্রগতি রিপোর্ট"]`,
		},
		{
			title:       "Class 4 English Version 2026",
			titleBn:     "চতুর্থ শ্রেণি ইংরেজি ভার্সন ২০২৬",
			description: "English version academic program for Class 4 students following national curriculum in English medium. Ideal for students planning to switch to English medium schools.",
			subject:     "All Subjects",
			teacher:     "Ayesha Tabassum",
			instructors: "Ayesha Tabassum, Asifuzzaman Reyad",
			classLevel:  "4",
			courseType:  "online",
			schedule:    "Sat-Tue, 5:00 PM - 6:30 PM",
			color:       "#EC4899",
			gradient:    "from-pink-500 to-pink-700",
			price:       599, oldPrice: 899, duration: "3 Months", badge: "New",
			studentsCount: 95, classesCount: 48, examsCount: 24, rating: 4.7, reviewsCount: 68,
			curriculum: `[{"month":"Month 1 — Core","topics":["Math: Number Operations","English: Reading Skills","Science: Living Things","ICT: Computer Basics"]},{"month":"Month 2 — Skills","topics":["Math: Geometry","English: Writing Workshop","Science: Materials","ICT: Internet Safety"]},{"month":"Month 3 — Application","topics":["Math: Problem Solving","English: Creative Writing","Science: Forces","ICT: Digital Literacy"]}]`,
			features:   `["English Medium Instruction","48 Live Classes","24 Exams","Interactive Labs","Study Materials","Certificate"]`,
		},
	}

	for _, c := range courses {
		_, err := DB.Exec(ctx,
			`INSERT INTO courses (title, title_bn, description, subject, teacher, instructors, class_level, type, schedule, color, gradient,
				price, old_price, duration, badge, students_count, classes_count, exams_count,
				rating, reviews_count, curriculum, features)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21::jsonb,$22::jsonb)`,
			c.title, c.titleBn, c.description, c.subject, c.teacher, c.instructors, c.classLevel, c.courseType, c.schedule,
			c.color, c.gradient, c.price, c.oldPrice, c.duration, c.badge,
			c.studentsCount, c.classesCount, c.examsCount, c.rating, c.reviewsCount,
			c.curriculum, c.features,
		)
		if err != nil {
			log.Printf("Failed to seed class course %q: %v", c.title, err)
		}
	}

	fmt.Println("Class 3-6 courses seeded")
}

func seedDemoHierarchy() {
	var count int
	_ = DB.QueryRow(context.Background(), `SELECT COUNT(*) FROM classes`).Scan(&count)
	if count > 0 {
		return
	}

	ctx := context.Background()

	insertOrGetBn := func(table, name, nameBn string) int {
		var id int
		err := DB.QueryRow(ctx, `SELECT id FROM `+table+` WHERE name = $1`, name).Scan(&id)
		if err == nil {
			return id
		}
		_ = DB.QueryRow(ctx, `INSERT INTO `+table+` (name, name_bn) VALUES ($1, $2) RETURNING id`, name, nameBn).Scan(&id)
		return id
	}

	insertBook := func(subjectID, classID int, name, nameBn, publisher string) int {
		var id int
		err := DB.QueryRow(ctx, `SELECT id FROM books WHERE name = $1 AND subject_id = $2 AND class_id = $3`, name, subjectID, classID).Scan(&id)
		if err == nil {
			return id
		}
		_ = DB.QueryRow(ctx, `INSERT INTO books (subject_id, class_id, name, name_bn, publisher) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
			subjectID, classID, name, nameBn, publisher).Scan(&id)
		return id
	}

	insertChapter := func(bookID int, name, nameBn string, orderIndex int) int {
		var id int
		err := DB.QueryRow(ctx, `SELECT id FROM chapters WHERE name = $1 AND book_id = $2`, name, bookID).Scan(&id)
		if err == nil {
			return id
		}
		_ = DB.QueryRow(ctx, `INSERT INTO chapters (book_id, name, name_bn, order_index) VALUES ($1,$2,$3,$4) RETURNING id`,
			bookID, name, nameBn, orderIndex).Scan(&id)
		return id
	}

	insertTopic := func(chapterID int, name, nameBn string, orderIndex int) int {
		var id int
		err := DB.QueryRow(ctx, `SELECT id FROM topics WHERE name = $1 AND chapter_id = $2`, name, chapterID).Scan(&id)
		if err == nil {
			return id
		}
		_ = DB.QueryRow(ctx, `INSERT INTO topics (chapter_id, name, name_bn, order_index) VALUES ($1,$2,$3,$4) RETURNING id`,
			chapterID, name, nameBn, orderIndex).Scan(&id)
		return id
	}

	// ========== Class 10 ==========
	class10ID := insertOrGetBn("classes", "Class 10", "দশম শ্রেণি")

	// Class 10 — Mathematics
	mathSubj := insertOrGetBn("subjects", "Mathematics", "গণিত")
	mathBook := insertBook(mathSubj, class10ID, "Mathematics", "গণিত", "NCTB")

	ch1 := insertChapter(mathBook, "Algebra", "বীজগণিত", 1)
	insertTopic(ch1, "Polynomials", "বহুপদী বীজগণিত", 1)
	insertTopic(ch1, "Equations", "সমীকরণ", 2)

	ch2 := insertChapter(mathBook, "Geometry", "জ্যামিতি", 2)
	insertTopic(ch2, "Triangles", "ত্রিভুজ", 1)
	insertTopic(ch2, "Circles", "বৃত্ত", 2)

	// Class 10 — Physics
	physSubj := insertOrGetBn("subjects", "Physics", "পদার্থবিজ্ঞান")
	physBook := insertBook(physSubj, class10ID, "Physics", "পদার্থবিজ্ঞান", "NCTB")

	ch3 := insertChapter(physBook, "Physical World", "ভৌত জগৎ", 1)
	insertTopic(ch3, "Units & Dimensions", "একক ও মাত্রা", 1)

	// ========== Class 8 ==========
	class8ID := insertOrGetBn("classes", "Class 8", "আটম শ্রেণি")

	engSubj := insertOrGetBn("subjects", "English", "ইংরেজি")
	engBook := insertBook(engSubj, class8ID, "English For Today", "English For Today", "NCTB")

	ch4 := insertChapter(engBook, "People", "People", 1)
	insertTopic(ch4, "Social Media", "Social Media", 1)
	insertTopic(ch4, "Stress Management", "Stress Management", 2)

	fmt.Println("Seeded demo hierarchy: 2 classes, 3 subjects, 3 books, 5 chapters, 9 topics")
}

func runFinanceMigration() {
	query := `
	CREATE TABLE IF NOT EXISTS expenses (
		id SERIAL PRIMARY KEY,
		category VARCHAR(100) NOT NULL,
		description TEXT NOT NULL,
		amount INT NOT NULL DEFAULT 0,
		date DATE DEFAULT CURRENT_DATE,
		notes TEXT DEFAULT '',
		created_by INT REFERENCES admin_users(id),
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_expenses_category ON expenses(category);
	CREATE INDEX IF NOT EXISTS idx_expenses_date ON expenses(date);
	`
	_, err := DB.Exec(context.Background(), query)
	if err != nil {
		log.Printf("Failed to run finance migration: %v", err)
	}
}

func seedDemoExpenses() {
	var count int
	_ = DB.QueryRow(context.Background(), `SELECT COUNT(*) FROM expenses`).Scan(&count)
	if count > 0 {
		return
	}

	type expense struct {
		category    string
		description string
		amount      int
		date        string
	}

	expenses := []expense{
		{"server", "Neon PostgreSQL hosting - September", 500, "2026-09-01"},
		{"sms", "BulkSMS BD credit recharge", 1200, "2026-09-05"},
		{"marketing", "Facebook ads campaign", 3000, "2026-09-10"},
		{"software", "Domain renewal edunova.com", 1500, "2026-08-15"},
		{"server", "Neon PostgreSQL hosting - August", 500, "2026-08-01"},
		{"sms", "BulkSMS BD credit recharge", 800, "2026-08-20"},
		{"salary", "Content writer payment", 5000, "2026-09-01"},
		{"marketing", "YouTube ad campaign", 2000, "2026-08-25"},
	}

	for _, e := range expenses {
		_, err := DB.Exec(
			context.Background(),
			`INSERT INTO expenses (category, description, amount, date) VALUES ($1, $2, $3, $4)`,
			e.category, e.description, e.amount, e.date,
		)
		if err != nil {
			log.Printf("Failed to seed expense %q: %v", e.description, err)
		}
	}

	fmt.Printf("Seeded %d demo expenses\n", len(expenses))
}

func runSMSMigration() {
	query := `
	CREATE TABLE IF NOT EXISTS sms_templates (
		id SERIAL PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		body TEXT NOT NULL,
		created_by INT REFERENCES admin_users(id),
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);
	`
	_, err := DB.Exec(context.Background(), query)
	if err != nil {
		log.Printf("Failed to run SMS migration: %v", err)
	}
}

func runBatchMigration() {
	query := `
	CREATE TABLE IF NOT EXISTS batches (
		id SERIAL PRIMARY KEY,
		course_id INT REFERENCES courses(id) ON DELETE CASCADE,
		name VARCHAR(255) NOT NULL,
		schedule VARCHAR(255) DEFAULT '',
		max_students INT DEFAULT 0,
		status VARCHAR(20) DEFAULT 'active',
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_batches_course ON batches(course_id);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_batches_name_lower ON batches (LOWER(name));

	ALTER TABLE enrollments ADD COLUMN IF NOT EXISTS batch_id INT REFERENCES batches(id) ON DELETE SET NULL;
	ALTER TABLE exams ADD COLUMN IF NOT EXISTS batch_id INT REFERENCES batches(id) ON DELETE SET NULL;

	-- Structured routine, replacing the old free-text-only schedule field.
	-- schedule is kept as a derived, human-readable string generated from
	-- these on every create/update, so existing displays (course cards,
	-- enrollment views) keep working unchanged.
	ALTER TABLE batches ADD COLUMN IF NOT EXISTS days TEXT[] DEFAULT '{}';
	ALTER TABLE batches ADD COLUMN IF NOT EXISTS start_time VARCHAR(10) DEFAULT '';
	ALTER TABLE batches ADD COLUMN IF NOT EXISTS end_time VARCHAR(10) DEFAULT '';

	-- Batches are now organized by class, not by course — course_id becomes
	-- an optional link (still used for enrollment/revenue tracking) rather
	-- than the primary key a batch is created under.
	ALTER TABLE batches ADD COLUMN IF NOT EXISTS class_level VARCHAR(20) NOT NULL DEFAULT '';
	CREATE INDEX IF NOT EXISTS idx_batches_class ON batches(class_level);
	ALTER TABLE batches ADD COLUMN IF NOT EXISTS admission_fee INT DEFAULT 0;
	ALTER TABLE batches ADD COLUMN IF NOT EXISTS note_fee INT DEFAULT 0;
	ALTER TABLE batches ADD COLUMN IF NOT EXISTS monthly_fee INT DEFAULT 0;
	ALTER TABLE batches ADD COLUMN IF NOT EXISTS shift VARCHAR(20) DEFAULT '';
	ALTER TABLE batches ADD COLUMN IF NOT EXISTS type VARCHAR(20) DEFAULT 'regular';
	ALTER TABLE batches ADD COLUMN IF NOT EXISTS year INT DEFAULT 0;
	CREATE INDEX IF NOT EXISTS idx_batches_year ON batches(year);

	-- Section (A/B/C/D, admin-chosen) replaces the old auto-scanned suffix
	-- letter, and code is a deterministic lookup number derived from
	-- year+class+type+shift+section. Old rows keep a blank code/section
	-- until edited; the partial index only enforces uniqueness once set.
	ALTER TABLE batches ADD COLUMN IF NOT EXISTS section VARCHAR(1) DEFAULT '';
	ALTER TABLE batches ADD COLUMN IF NOT EXISTS code VARCHAR(10) DEFAULT '';
	CREATE UNIQUE INDEX IF NOT EXISTS idx_batches_code ON batches (code) WHERE code <> '';
	`
	_, err := DB.Exec(context.Background(), query)
	if err != nil {
		log.Printf("Failed to run batch migration: %v", err)
	}
}

func seedDemoBatches() {
	var count int
	_ = DB.QueryRow(context.Background(), `SELECT COUNT(*) FROM batches`).Scan(&count)
	if count > 0 {
		return
	}

	ctx := context.Background()

	// Find SSC 2027 Science course
	var courseID int
	err := DB.QueryRow(ctx, `SELECT id FROM courses WHERE title = $1`, "SSC 2027 Science Group").Scan(&courseID)
	if err == nil {
		DB.Exec(ctx, `INSERT INTO batches (course_id, name, schedule, max_students, status, class_level, shift, type) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			courseID, "Morning Batch", "Mon-Wed-Fri 8:00 AM", 40, "active", "Class 10", "Morning", "Regular")
		DB.Exec(ctx, `INSERT INTO batches (course_id, name, schedule, max_students, status, class_level, shift, type) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			courseID, "Evening Batch", "Tue-Thu-Sat 6:00 PM", 40, "active", "Class 10", "Evening", "Regular")
	}

	// Find Class 8 Academic course
	var courseID2 int
	err = DB.QueryRow(ctx, `SELECT id FROM courses WHERE title = $1`, "Class 8 Complete Academic").Scan(&courseID2)
	if err == nil {
		DB.Exec(ctx, `INSERT INTO batches (course_id, name, schedule, max_students, status, class_level, shift, type) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			courseID2, "Regular Batch", "Sat-Sun 10:00 AM", 35, "active", "Class 8", "Weekend", "Regular")
	}

	fmt.Println("Seeded demo batches: 3 batches across 2 courses")
}

func runAttendanceMigration() {
	ctx := context.Background()

	// Add parent/academic fields to users table
	userAlters := []string{
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS father_name VARCHAR(255) DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS father_mobile VARCHAR(20) DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS mother_name VARCHAR(255) DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS mother_mobile VARCHAR(20) DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS notification_mobile VARCHAR(20) DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS sms_opt_in BOOLEAN NOT NULL DEFAULT false`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS last_result_sms_on DATE`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS gender VARCHAR(20) DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS religion VARCHAR(50) DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS student_class VARCHAR(50) DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS shift VARCHAR(20) DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS school VARCHAR(255) DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS address TEXT DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS present_address TEXT DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS permanent_address TEXT DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS photo_url TEXT DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS google_id VARCHAR(255) DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS email VARCHAR(255) DEFAULT ''`,
	}
	for _, q := range userAlters {
		_, _ = DB.Exec(ctx, q)
	}

	// Carry forward any address already captured under the old single-field
	// column so existing users aren't bounced back into profile setup.
	_, _ = DB.Exec(ctx, `UPDATE users SET present_address = address, permanent_address = address
		WHERE present_address = '' AND address <> ''`)

	// Create attendance table
	_, err := DB.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS attendance (
		id SERIAL PRIMARY KEY,
		student_id INT REFERENCES users(id) ON DELETE CASCADE,
		date DATE NOT NULL,
		status VARCHAR(20) NOT NULL DEFAULT 'present',
		marked_by INT REFERENCES admin_users(id),
		notes TEXT DEFAULT '',
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		UNIQUE(student_id, date)
	)`)
	if err != nil {
		log.Printf("Warning: failed to create attendance table: %v", err)
	}

	// Create holidays table
	_, err = DB.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS holidays (
		id SERIAL PRIMARY KEY,
		date DATE NOT NULL UNIQUE,
		reason VARCHAR(255) DEFAULT '',
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`)
	if err != nil {
		log.Printf("Warning: failed to create holidays table: %v", err)
	}

	// Create device tokens table for FCM push notifications
	_, err = DB.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS device_tokens (
		id SERIAL PRIMARY KEY,
		user_id INT REFERENCES users(id) ON DELETE CASCADE,
		token TEXT NOT NULL,
		platform VARCHAR(20) DEFAULT 'android',
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		UNIQUE(user_id, token)
	)`)
	if err != nil {
		log.Printf("Warning: failed to create device_tokens table: %v", err)
	}

	fmt.Println("Attendance migration completed")
}

func runDoubtTrackerMigration() {
	ctx := context.Background()
	_, err := DB.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS doubts (
		id SERIAL PRIMARY KEY,
		student_id INT REFERENCES users(id) ON DELETE CASCADE,
		question_text TEXT NOT NULL,
		subject VARCHAR(255) DEFAULT '',
		chapter VARCHAR(255) DEFAULT '',
		image_url TEXT DEFAULT '',
		status VARCHAR(20) DEFAULT 'pending',
		resolution TEXT DEFAULT '',
		resolved_by INT REFERENCES admin_users(id),
		resolved_at TIMESTAMP WITH TIME ZONE,
		parent_notified BOOLEAN DEFAULT FALSE,
		student_notified BOOLEAN DEFAULT FALSE,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`)
	if err != nil {
		log.Printf("Warning: failed to create doubts table: %v", err)
	}
	fmt.Println("Doubt tracker migration completed")
}

func runCalendarMigration() {
	ctx := context.Background()
	_, err := DB.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS calendar_events (
		id SERIAL PRIMARY KEY,
		title VARCHAR(255) NOT NULL,
		description TEXT DEFAULT '',
		event_type VARCHAR(50) NOT NULL,
		date DATE NOT NULL,
		end_date DATE,
		course_id INT REFERENCES courses(id) ON DELETE SET NULL,
		batch_id INT REFERENCES batches(id) ON DELETE SET NULL,
		color VARCHAR(20) DEFAULT '#6366F1',
		is_auto BOOLEAN DEFAULT FALSE,
		notification_sent BOOLEAN DEFAULT FALSE,
		created_by INT REFERENCES admin_users(id),
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`)
	if err != nil {
		log.Printf("Warning: failed to create calendar_events table: %v", err)
	}
	fmt.Println("Calendar migration completed")
}

func runLessonsMigration() {
	ctx := context.Background()
	_, err := DB.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS lessons (
		id SERIAL PRIMARY KEY,
		course_id INT REFERENCES courses(id) ON DELETE CASCADE,
		batch_id INT REFERENCES batches(id) ON DELETE SET NULL,
		title VARCHAR(255) NOT NULL,
		description TEXT DEFAULT '',
		subject VARCHAR(255) DEFAULT '',
		chapter VARCHAR(255) DEFAULT '',
		lesson_date DATE NOT NULL,
		teacher_notes TEXT DEFAULT '',
		created_by INT REFERENCES admin_users(id),
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`)
	if err != nil {
		log.Printf("Warning: failed to create lessons table: %v", err)
	}
	// kind: lesson (class note), video, homework; link_url is an optional
	// YouTube/Drive/etc. link for the guardian-facing study feed.
	for _, q := range []string{
		`ALTER TABLE lessons ADD COLUMN IF NOT EXISTS kind VARCHAR(20) NOT NULL DEFAULT 'lesson'`,
		`ALTER TABLE lessons ADD COLUMN IF NOT EXISTS link_url TEXT DEFAULT ''`,
		// Curriculum link. chapter/topic text stay as a snapshot of the names at
		// posting time, so renaming or deleting curriculum never rewrites history.
		`ALTER TABLE lessons ADD COLUMN IF NOT EXISTS topic VARCHAR(255) DEFAULT ''`,
		`ALTER TABLE lessons ADD COLUMN IF NOT EXISTS chapter_id INT REFERENCES chapters(id) ON DELETE SET NULL`,
		`ALTER TABLE lessons ADD COLUMN IF NOT EXISTS topic_id INT REFERENCES topics(id) ON DELETE SET NULL`,
		`CREATE INDEX IF NOT EXISTS idx_lessons_batch_date ON lessons(batch_id, lesson_date DESC)`,
	} {
		if _, err := DB.Exec(ctx, q); err != nil {
			log.Printf("Warning: failed to alter lessons table: %v", err)
		}
	}
	fmt.Println("Lessons migration completed")
}

func runPaymentsMigration() {
	ctx := context.Background()
	_, err := DB.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS payments (
		id SERIAL PRIMARY KEY,
		user_id INT REFERENCES users(id) ON DELETE CASCADE,
		enrollment_id INT REFERENCES enrollments(id) ON DELETE SET NULL,
		course_id INT REFERENCES courses(id) ON DELETE SET NULL,
		amount DECIMAL(10,2) NOT NULL,
		method VARCHAR(50) NOT NULL,
		transaction_id VARCHAR(255) DEFAULT '',
		sender_number VARCHAR(20) DEFAULT '',
		receiver_number VARCHAR(20) DEFAULT '',
		status VARCHAR(20) DEFAULT 'pending',
		receipt_number VARCHAR(100) DEFAULT '',
		month VARCHAR(20) DEFAULT '',
		year INT DEFAULT 0,
		notes TEXT DEFAULT '',
		verified_by INT REFERENCES admin_users(id),
		verified_at TIMESTAMP WITH TIME ZONE,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`)
	if err != nil {
		log.Printf("Warning: failed to create payments table: %v", err)
	}
	_, _ = DB.Exec(ctx, `ALTER TABLE payments ADD COLUMN IF NOT EXISTS batch_id INT REFERENCES batches(id) ON DELETE SET NULL`)
	_, _ = DB.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_payments_batch_id ON payments(batch_id)`)
	fmt.Println("Payments migration completed")
}

// runInvoicesMigration creates the immutable invoice ledger. Source rows are
// ON DELETE SET NULL so an invoice outlives a deleted enrollment/payment.
func runInvoicesMigration() {
	ctx := context.Background()
	for _, q := range []string{
		`CREATE TABLE IF NOT EXISTS invoices (
			id SERIAL PRIMARY KEY,
			kind VARCHAR(20) NOT NULL,
			number VARCHAR(100) NOT NULL,
			enrollment_id INT REFERENCES enrollments(id) ON DELETE SET NULL,
			payment_id INT REFERENCES payments(id) ON DELETE SET NULL,
			user_id INT REFERENCES users(id) ON DELETE SET NULL,
			batch_id INT REFERENCES batches(id) ON DELETE SET NULL,
			total DECIMAL(10,2) NOT NULL DEFAULT 0,
			snapshot JSONB NOT NULL,
			issued_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_invoices_enrollment ON invoices(enrollment_id) WHERE kind = 'admission'`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_invoices_payment ON invoices(payment_id) WHERE kind = 'payment'`,
		`CREATE INDEX IF NOT EXISTS idx_invoices_user ON invoices(user_id, issued_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_invoices_number ON invoices(number)`,
	} {
		if _, err := DB.Exec(ctx, q); err != nil {
			log.Printf("Warning: failed to run invoices migration: %v", err)
		}
	}
	fmt.Println("Invoices migration completed")
}

func runParentingHubMigration() {
	ctx := context.Background()
	_, err := DB.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS articles (
		id SERIAL PRIMARY KEY,
		title VARCHAR(255) NOT NULL,
		content TEXT NOT NULL,
		category VARCHAR(100) DEFAULT 'general',
		video_url TEXT DEFAULT '',
		image_url TEXT DEFAULT '',
		is_published BOOLEAN DEFAULT TRUE,
		created_by INT REFERENCES admin_users(id),
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`)
	if err != nil {
		log.Printf("Warning: failed to create articles table: %v", err)
	}
	fmt.Println("Parenting hub migration completed")

	// Ensure unique title on articles for ON CONFLICT DO NOTHING
	_, _ = DB.Exec(ctx, `DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'articles_title_unique') THEN
			ALTER TABLE articles ADD CONSTRAINT articles_title_unique UNIQUE (title);
		END IF;
	END $$`)
}

func seedParentingHubArticles() {
	ctx := context.Background()

	// Fix old category names to be SEO-friendly
	_, _ = DB.Exec(ctx, `UPDATE articles SET category = 'child-development' WHERE category = 'child_development'`)

	var count int
	err := DB.QueryRow(ctx, "SELECT COUNT(*) FROM articles").Scan(&count)
	if err != nil || count >= 32 {
		return
	}

	articles := []struct {
		Title    string
		Content  string
		Category string
		ImageURL string
	}{
		{
			Title:    "সন্তানের মোবাইল আসক্তি কমানোর ৭টি সহজ কৌশল",
			Category: "screen_time",
			ImageURL: "https://images.unsplash.com/photo-1563013544-824ae1b704d3?w=800",
			Content: `বর্তমান যুগে মোবাইল ফোন আমাদের জীবনের অঙ্গ হয়ে উঠেছে। কিন্তু, শিশুদের হাতে যদি নিয়ন্ত্রণহীনভাবে মোবাইল থাকে, তবে তা শারীরিক, মানসিক এবং সামাজিক বিকাশে নেতিবাচক প্রভাব ফেলে। দীর্ঘ সময় ধরে মোবাইল ব্যবহারের ফলে শিশুদের মধ্যে একাকীত্ব, ঘুমের সমস্যা, চোখের সমস্যা ও আচরণগত পরিবর্তন দেখা দিতে পারে।

১. রুটিন তৈরি করুন — শিশুরা নিয়ম মেনে চলতে ভালবাসে। দিনে কতক্ষণ মোবাইল ব্যবহার করা যাবে, তা নির্দিষ্ট করে দিন। যেমন– স্কুল শেষে ৩০ মিনিট, পড়া শেষের পর ১৫ মিনিট।

২. বিকল্প ব্যবস্থা করুন — শুধু "না" বললে কাজ হবে না। গল্পের বই, আঁকাআঁকি, ব্লকস, পাজল, বাইরের খেলাধুলা বা প্যারেন্ট-চাইল্ড অ্যাক্টিভিটিতে শিশুদের উৎসাহিত করুন।

৩. নিজের ব্যবহারেও সংযম আনুন — আপনি যদি সারাক্ষণ ফোনে থাকেন, তবে শিশু সেটাই শিখবে। শিশুদের সামনে মোবাইল ব্যবহার কমান এবং "ডিভাইস ফ্রি টাইম" চালু করুন।

৪. শিক্ষামূলক অ্যাপ বেছে নিন — ফোন ব্যবহার একেবারে বন্ধ করা না গেলে, অন্তত শিক্ষামূলক ও বুদ্ধিবৃত্তিক অ্যাপ বা ভিডিও দেখতে উৎসাহিত করুন।

৫. মোবাইলকে পুরস্কার হিসেবে ব্যবহার করবেন না — অনেক সময় দেখা যায়, অভিভাবকরা ভালো কাজের পুরস্কার হিসেবে মোবাইল ব্যবহার করতে দেন। এটি একটি ভুল বার্তা দেয়।

৬. শারীরিক ও সামাজিক অ্যাক্টিভিটিতে অংশগ্রহণ — বন্ধুদের সঙ্গে সময় কাটানো, খেলাধুলা, ঘুরতে যাওয়া, ফ্যামিলি টাইম– এগুলি শিশুদের মোবাইল থেকে দূরে রাখে।

৭. ধৈর্য ধরুন — শিশুদের অভ্যাস রাতারাতি বদলায় না। ধীরে ধীরে পরিবর্তনের দিকে নিয়ে যান। প্রয়োজনে শিশুর সঙ্গে কথা বলুন, তাদের বোঝার চেষ্টা করুন।

আজকের দিনে শিশুকে মোবাইল থেকে পুরোপুরি দূরে রাখা সম্ভব নয়। কিন্তু, নিয়ন্ত্রণ করাটাই আসল কাজ। সন্তানের সঙ্গে বেশি সময় কাটান, সম্পর্ক গড়ে তুলুন, তাহলেই তার মোবাইলের প্রতি নির্ভরতা ধীরে ধীরে কমে আসবে।`,
		},
		{
			Title:    "কিশোর বয়েসে সন্তানের সাথে ভালো সম্পর্ক গড়ে তোলার উপায়",
			Category: "teen_parenting",
			ImageURL: "https://images.unsplash.com/photo-1536337005238-94b997371b40?w=800",
			Content: `শিশুরা যখন বড় হতে শুরু করে, তখন প্রায়শই বাবা-মা এবং সন্তানদের মধ্যে দূরত্ব আরও গভীর হতে শুরু করে। শিশুদের মধ্যে অনেক ধরণের শারীরিক ও মানসিক পরিবর্তন ঘটতে শুরু করে। এই পরিবর্তনের কারণে শিশুদের আচরণও পরিবর্তিত হতে শুরু করে।

বাচ্চাদের কথা শোনার চেষ্টা করুন — অভিভাবকদের তাদের সন্তানদের বাধা না দিয়ে তাদের কথা শোনতে শেখা উচিত। যখন তারা বিচারিত বোধ করবেন না, তখন তারা আরও ভালোভাবে নিজেদের প্রকাশ করতে পারবে।

বিশ্বস্ত সম্পর্ক তৈরি করা — আপনি যদি আপনার কিশোর সন্তানকে আপনার কথা শোনাতে চান, তাহলে আপনাকেও কিন্তু তার কথা শোনার সময় ও ধৈর্য থাকতে হবে। একটি খোলা সম্পর্ক রাখুন সন্তানের সাথে।

সহানুভূতিশীল হোয়েন — মনে রাখবেন, আপনিও কোনো একসময় কিশোর ছিলেন। আপনার কিশোর আচরণের অনুভূতির অভাবনীয় ব্যাপারগুলো কল্পনা করুন এবং ভাবুন যে আপনার কিশোর সন্তানটি কেমন অনুভব করছে।

সন্তানের প্রতি শ্রদ্ধাশীল হোয়েন — সন্তানের ব্যক্তিত্ব, ধারণা, মতামত এবং আবেগ অনুভব করুন এবং সম্মান দিন। তাদের বন্ধুদের সামনে কখনো নিন্দা করবেন না।

সংযম বজায় রাখুন — মায়েরও ধৈর্য ধরে রাখা প্রয়োজন। কিশোর-কিশোরীদের সাথে মায়ের শান্ত থাকা উচিত। আপনার শক্তি কথার চেয়ে শিশুর ওপর বেশি প্রভাব ফেলবে।

কিশোর-কিশোরীদের স্বাধীনতা, গোপনীয়তা, উত্তেজনা এবং নিয়ন্ত্রণের প্রয়োজন। যদি তারা ঘরে এই জিনিসগুলো না পায়, তাহলে তারা বাইরে এই জিনিসগুলো খুঁজতে শুরু করে। তাই পরিবারেই তাদের এই প্রয়োজনগুলো পূরণ করুন।`,
		},
		{
			Title:    "পরীক্ষার চাপ থেকে সন্তানকে মুক্ত করার কার্যকর উপায়",
			Category: "exam_stress",
			ImageURL: "https://images.unsplash.com/photo-1434030216411-0b793f4b4173?w=800",
			Content: `পরীক্ষার চাপ আমাদের দেশের প্রায় প্রতিটি শিক্ষার্থীর জীবনেরই একটি বাস্তবতা। SSC, HSC কিংবা বিশ্ববিদ্যালয় ভর্তি পরীক্ষার আগে বুক ধড়ফড় করা, ঘুম না আসা, বারবার মনে হওয়া "আমি পারব না" — এই অনুভূতিগুলো অস্বাভাবিক কিছু নয়।

পরীক্ষার চাপ শুধু পড়া মনে থাকবে কি না, সেই ভয় থেকে আসে। আমাদের সামাজিক প্রেক্ষাপটে এর পেছনে আরও অনেক স্তর কাজ করে। সবাইকে একই রকম রেজাল্ট, একই বিষয়, একই ক্যারিয়ারের দিকে ঠেলে দেওয়া হয়।

কীভাবে চাপ কমাবেন:

ছোট ছোট লক্ষ্য ঠিক করুন — পুরো সিলেবাস একসঙ্গে ভাবলে মনে হয় পাহাড়ের মতো। বদলে প্রতিদিনের জন্য ছোট, বাস্তবসম্মত লক্ষ্য ঠিক করুন।

পর্যাপ্ত ঘুম দিন — রাত জেগে পড়ে পরদিন ক্লান্ত থাকার চেয়ে পর্যাপ্ত ঘুম অনেক বেশি কার্যকর।

গভীর শ্বাসের অভ্যাস শেখান — ধীরে ধীরে নাক দিয়ে শ্বাস নিন, কয়েক সেকেন্ড ধরে রাখুন, তারপর আস্তে ছেড়ে দিন। বুক ধড়ফড় বা প্যানিকের মুহূর্তে এটি স্নায়ুতন্ত্রকে শান্ত করতে সাহায্য করে।

তুলনা বন্ধ করুন — "রেজাল্ট যাই হোক, তুমি আমাদের কাছে গুরুত্বপূর্ণ" — এই আশ্বাস দিলে সন্তানের ভয় অনেকটাই কমে আসে।

প্যানিক অ্যাটাকের সময় — বুক ধড়ফড়, ঘাম হওয়া, শরীরে অস্বস্তি দেখা দিলে গ্রাউন্ডিং টেকনিক ব্যবহার করুন: "সামনে কোন ৫ টি জিনিস দেখতে পাচ্ছ, বলো", "কোন কোন আওয়াজ শুনতে পাচ্ছ"। এই প্রশ্নের উত্তর দিতে গেলেই মন ভয় থেকে সরে যাবে।

পরীক্ষার আগের রাতে অন্তত ৭-৮ ঘণ্টা ঘুমোনো প্রয়োজন। শোয়ার অন্তত এক ঘণ্টা আগে বই ও মোবাইল সরিয়ে রাখুন।

পরীক্ষা শেষ হওয়ার পর সহপাঠীদের সঙ্গে উত্তর নিয়ে আলোচনা করা থেকে বিরত থাকুন। কারণ, যদি ভুল উত্তর দেখেন, তা আপনাকে হতাশ করতে পারে এবং পরের পরীক্ষার প্রস্তুতিও প্রভাবিত হতে পারে।`,
		},
		{
			Title:    "কিশোরদের মানসিক স্বাস্থ্য সুস্থ রাখার অভিভাবক গাইড",
			Category: "mental_health",
			ImageURL: "https://images.unsplash.com/photo-1544027993-37dbfe43562a?w=800",
			Content: `একটি শিশুর মানসিক সুস্থতা তাদের শারীরিক সুস্থতার মতো একইভাবে গুরুত্বপূর্ণ। আপনার সঙ্গে আপনার সন্তানের সম্পর্ক, তার প্রতি আপনার ভালবাসা, পাশে থাকা — সবকিছুই তার মানসিক স্বাস্থ্যের ওপর একটি ইতিবাচক প্রভাব ফেলে।

কমিউনিকেট করতে উত্সাহিত করুন — সন্তানের মঙ্গলের জন্য এটি খুবই গুরুত্বপূর্ণ। তাদের সমস্ত রকমের সমস্যা এবং সংগ্রামে সর্বদা তাদের পাশে থাকুন।

নিজস্ব জায়গা দিন — টিন এজার্সদের স্বাবলম্বী হতে চাওয়াটা খুবই স্বাভাবিক। তাদের নিজস্ব একটা স্পেস দিন এবং পারলে তাদের সমস্যাগুলোর সমাধান খুঁজুন।

সৎ হোয়েন — সন্তানদের প্রতি সৎ হোয়েন এবং তাদের বুঝিয়ে দিন যে আপনিও তাদের মঙ্গল নিয়ে ভাবেন।

মেডিটেশন ও মননশীলতার সাথে পরিচয় ঘটান — প্রতিদিনের অনুশীলন তাদের স্ট্রেস এবং উদ্বেগ কমাতে সাহায্য করবে এবং তাদের আরও আত্ম-সচেতন এবং আত্মবিশ্বাসী হতে সাহায্য করবে।

স্বাস্থ্যকর খাবার — খাবার এবং সুস্থতার মধ্যে গভীর সম্পর্ক রয়েছে। তাদের প্রচুর পরিমাণে স্বাস্থ্যকর শাকসবজি এবং ফল খেতে উত্সাহিত করুন।

চিন্তাশীল হতে শেখান — তাদের দায়িত্বশীল হতে এবং সিদ্ধান্ত নিতে শেখান। তাদের আশ্বস্ত করুন যে তারা জীবনের যে কোন চ্যালেঞ্জের সাথে মোকাবেলা করতে পারে।

চলাফেরা করতে উৎসাহিত করুন — সোশ্যাল মিডিয়ার অতিরিক্ত ব্যবহার প্রায়ই কিশোর-কিশোরীদের শারীরিকভাবে নিষ্ক্রিয় করে তোলে। প্রতিদিন ব্যায়াম করতে বা সক্রিয় জীবনধারা অনুসরণ করতে অনুপ্রাণিত করুন।

পরিবার এবং সন্তানদের সঙ্গে সুন্দর সময় কাটান, তাদের নিজেদের অনুভূতিগুলো আপনার সঙ্গে শেয়ার করতে বলুন এবং তাদের বুঝিয়ে দিন যে জীবনে কঠিন লড়াইয়ে তারা এক নয়।`,
		},
		{
			Title:    "বাচ্চাদের পড়ার অভ্যাস গড়ে তোলার ৫টি কৌশল",
			Category: "study_habits",
			ImageURL: "https://images.unsplash.com/photo-1503676260728-1c00da094a0b?w=800",
			Content: `প্রতিটি বাবা-মাই চান তাদের সন্তান পড়াশোনায় মেধাবী হোক। কিন্তু পড়ার অভ্যাস গড়ে তোলা সহজ কাজ নয়। এখানে কিছু সহজ ও কার্যকর কৌশল দেওয়া হলো:

১. নিয়মিত সময়সূচি তৈরি করুন — প্রতিদিন একটি নির্দিষ্ট সময়ে পড়ার অভ্যাস গড়ে তুলুন। যেমন বিকেলে ৪টা থেকে ৫টা পর্যন্ত। শিশুরা রুটিন মেনে চলতে ভালোবাসে।

২. পড়ার জায়গা সুন্দর করুন — একটি পরিষ্কার, আলোকিত ও শান্ত জায়গায় পড়ার ব্যবস্থা করুন। মোবাইল ফোন, টিভি বা অন্যান্য বিভ্রান্তিকর জিনিস দূরে রাখুন।

৩. ছোট ছোট লক্ষ্য ঠিক করুন — পুরো অধ্যায় একসাথে পড়ার চেয়ে ছোট ছোট ভাগে ভাগ করুন। প্রতিটি ভাগ শেষ করার পর বিশ্রাম দিন। এতে শিশু হতাশ হবে না।

৪. প্রশংসা করুন — ছোট ছোট সাফল্যের প্রশংসা করলেও তাদের আত্মবিশ্বাস বাড়বে। খেয়াল রাখবেন প্রশংসা যেন আন্তরিক ও খাঁটি হয়। বকাঝকার পরিবর্তে স্নেহভরে বিষয়টি বুঝিয়ে বলুন।

৫. তুলনা করবেন না — প্রতিটি শিশুই অনন্য। তাদের চিন্তাভাবনা, ক্ষমতা এবং আগ্রহও ভিন্ন। আপনার সন্তানকে কখনোই অন্য শিশুদের সাথে তুলনা করবেন না। এটি তাদের আত্মবিশ্বাসকে ক্ষুণ্ণ করতে পারে।

অনেক সময় আমরা ধরে নিই যে সন্তানকে ভালো করে বড় করার জন্য প্রচুর আর্থিক ব্যয়ের প্রয়োজন। কিন্তু সবচেয়ে গুরুত্বপূর্ণ বিষয়গুলো হলো আপনার সময়, বোঝাপড়া এবং সঠিক দৃষ্টিভঙ্গি। ছোট ছোট পরিবর্তনও আপনার সন্তানের চিন্তাভাবনা, আচরণ এবং ভবিষ্যতের ওপর উল্লেখযোগ্য প্রভাব ফেলতে পারে।`,
		},
		{
			Title:    "সন্তানকে মোবাইল দিয়ে খাওয়ানো — এই ভুলটি বন্ধ করুন",
			Category: "screen_time",
			ImageURL: "https://images.unsplash.com/photo-1588196749597-9ff075ee6b5b?w=800",
			Content: `অনেক সময়, বাবা-মা সন্তানকে শান্ত রাখার জন্য তাদের হাতে ফোন দিয়ে দেন। বাচ্চাকে খাওয়ানোর সময়ও ফোনের সাহায্য নেওয়া হয়। কিন্তু এই অভ্যাস অত্যন্ত ক্ষতিকর। এর ফলেই ছোটদের মধ্যে দেখা দেয় ফোনের আসক্তি।

বাচ্চাকে খাওয়ানোর সময় মোবাইল নয় — অনেক বাবা-মা সন্তানকে খাওয়ানোর সময় মোবাইল ধরিয়ে দেন, যাতে খেতে বসে থাকে। কিন্তু এই অভ্যাস ক্ষতিকর। এতে খাওয়ার প্রতি মনোযোগ থাকে না, খাবারের স্বাদ বা আগ্রহও কমে যায়।

স্ক্রিন টাইমের নিয়ম নিশ্চিত করুন — ফোন ব্যবহারের ক্ষেত্রে কিছু নিয়ম তৈরি করুন। সবার আগে রাত ৮টার পর ফোন ব্যবহার না করার নিয়ম লাগু করুন। এবং এই নিয়ম সবার আগে আপনাকে পালন করতে হবে। তবেই আপনার সন্তান এটি শিখবে।

৬ বছর বা তার বেশি বয়সের শিশুদের: দৈনিক ২ ঘণ্টার বেশি নয়। পড়াশোনা এবং বিনোদন মিলিয়ে এই সময়ের বেশি ব্যবহার করতে দেবেন না।

২-৫ বছরের শিশুদের: দৈনিক ১ ঘণ্টার বেশি নয়। সেটিও শুধু পড়াশোনা বা কিছু শেখানোর জন্য।

১৮ মাসের নীচে: এই বয়েসর শিশুদের হাতে কখনও মোবাইল ফোন দেওয়া উচিত নয়। তাতে শিশুর চোখ এবং মস্তিষ্কের ভয়ানয় ক্ষতি হতে পারে।

ফোন ব্যবহারের পর চোখ ও মস্তিস্ককে আরাম দিন — এক টানা ফোন ব্যবহার একদমই ভালো নয়। ফোন ব্যবহারের পর চোখ ও মস্তিস্ককে আরাম দেওয়া আবশ্যক।

খাওয়ার সময় পরিবার একসঙ্গে গল্প করুন। তাতে খাবারও শেষ হবে, সময়ও আনন্দময় হবে। ধীরে ধীরে স্ক্রিন টাইম কমান — একেবারে হঠাৎ করে মোবাইল বন্ধ করে দেবেন না। এতে শিশু আরো জেদি হতে পারে।`,
		},
		{
			Title:    "টিনেজ সন্তানের সাথে বন্ধুত্বপূর্ণ সম্পর্ক গড়ে তোলার গাইড",
			Category: "teen_parenting",
			ImageURL: "https://images.unsplash.com/photo-1529156069898-49953e39b3ac?w=800",
			Content: `পরিবার, মা-বাবা কিন্তু সন্তানের সবচেয়ে আস্থার জায়গা। এই কথাটি আপনার কিশোর বয়সের সন্তানের কাছে বোধগম্য হবে না, এটাই স্বাভাবিক। শুধু কথায় না বরং কথায় ও কাজের সংমিশ্রণে সন্তানকে এই ব্যাপারটি বোঝান।

বিশ্বস্ত সম্পর্ক তৈরি করুন — ট্রাস্ট বা বিশ্বাস যে কোনো সম্পর্কের জন্যই গুরুত্বপূর্ণ। আপনি যদি আপনার কিশোর সন্তানকে আপনার কথা শোনাতে চান, তাহলে আপনাকেও কিন্তু তার কথা শোনার সময় ও ধৈর্য থাকতে হবে।

সহানুভূতিশীল হোয়েন — মনে রাখবেন, আপনিও কোনো একসময় কিশোর ছিলেন। আপনার কিশোর আচরণের অনুভূতির অভাবনীয় ব্যাপারগুলো কল্পনা করুন এবং ভাবুন যে আপনার কিশোর সন্তানটি কেমন অনুভব করছে।

সন্তানের প্রতি শ্রদ্ধাশীল হোয়েন — সন্তানের ব্যক্তিত্ব, ধারণা, মতামত এবং আবেগ অনুভব করুন এবং সম্মান দিন। তাদের বন্ধুদের সামনে, কেউ সামনে না থাকলেও তাদের নিন্দা করবেন না এবং সবচেয়ে গুরুত্বপূর্ণ ব্যাপার, তাদের মতামতকে তুচ্ছ বা সমালোচনা করবেন না।

স্বাধীনতার সুযোগ দিন — বেশিরভাগ কিশোর কিশোরী নিজেদের স্বয়ংসম্পূর্ণ ভাবতে পছন্দ করে এবং মনে করে কারো সাহায্যের প্রয়োজন নেই। তাদের জানা প্রয়োজন যে আপনি তাদের সাহায্য করতে ইচ্ছুক। তাদের যে কোনো প্রয়োজনে আপনি পাশে আছেন বন্ধুর মতো।

সন্তানের নিরাপত্তার কথা চিন্তা করে আপনি কখনো কঠোর হতে পারেন কিন্তু কঠোরতার মাত্রা ছাড়িয়ে গেলে সন্তান ভুল বুঝবে আপনাকে। আপনার অতিরিক্ত শৃঙ্খলা এবং কঠোরতা দেখে তারা বিশ্বাস করতে পারে যে আপনি শুধুমাত্র তাদের জন্য জীবনকে কঠিনই করতে চান। তাদের বলুন আপনি তাদের ভালোবাসেন এবং জীবনে শৃঙ্খলা থাকা দরকার সবারই।

কিশোর-কিশোরীদের স্বাধীনতা, গোপনীয়তা, উত্তেজনা এবং নিয়ন্ত্রণের প্রয়োজন। যদি তারা ঘরে এই জিনিসগুলো না পায়, তাহলে তারা বাইরে এই জিনিসগুলো খুঁজতে শুরু করে।`,
		},
		{
			Title:    "শিশুদের মধ্যে আত্মবিশ্বাস গড়ে তোলার ৬টি উপায়",
			Category: "child-development",
			ImageURL: "https://images.unsplash.com/photo-1503454537195-1dcabb73ffb9?w=800",
			Content: `আত্মবিশ্বাসী সন্তান জীবনের প্রতিটি চ্যালেঞ্জ মোকাবেলা করতে পারে। কিন্তু আত্মবিশ্বাস কোথা থেকে আসে? এটি মূলত পরিবার থেকেই শেখে। এখানে কিছু সহজ উপায় দেওয়া হলো:

১. দৈনিক ১০-১৫ মিনিট সময় দিন — আজকের ব্যস্ত জীবনে আমরা প্রায়শই আমাদের সন্তানদের সাথে বসে কথা বলতে ভুলে যাই। প্রতিদিন ১০-১৫ মিনিট সময় নিয়ে আপনার সন্তানের সাথে মন খুলে কথা বলুন।

২. প্রশ্নের উত্তর দিন — শিশুরা সবকিছু জানতে চায়। কখনও কখনও আমরা তাদের প্রশ্নে বিরক্ত হয়ে তাদের চুপ করানোর চেষ্টা করি, যা ভুল। যখন কোনো শিশু প্রশ্ন করে, তখন মনোযোগ দিয়ে শুনুন এবং ব্যাখ্যা করার চেষ্টা করুন।

৩. স্ক্রিন টাইম সীমিত করুন — মোবাইল ফোন, টিভি এবং ট্যাবলেট শিশুদের জীবনের অবিচ্ছেদ্য অংশ হয়ে উঠেছে, কিন্তু অতিরিক্ত স্ক্রিন টাইম তাদের মস্তিষ্ক ও স্বাস্থ্য উভয়ের জন্যই ক্ষতিকর। এর পরিবর্তে, তাদের বই পড়তে, ছবি আঁকতে, গেম খেলতে বা ধাঁধা সমাধান করতে উৎসাহিত করুন।

৪. প্রশংসা করুন — প্রত্যেক শিশুই তার কাজের জন্য প্রশংসা পেতে চায়। ছোট ছোট সাফল্যের প্রশংসা করলেও তাদের আত্মবিশ্বাস বাড়বে।

৫. দায়িত্ব দিন — শিশুদের ছোট ছোট দায়িত্ব দেওয়া জরুরি, যেমন নিজেদের ব্যাগ গোছানো, বইপত্র গুছিয়ে রাখা বা নিজেদের ঘর পরিষ্কার করা। এর মাধ্যমে তাদের মধ্যে শৃঙ্খলা ও দায়িত্ববোধ গড়ে ওঠে।

৬. তুলনা করবেন না — প্রতিটি শিশুই অনন্য। তাদের চিন্তাভাবনা, ক্ষমতা এবং আগ্রহও ভিন্ন। আপনার সন্তানকে কখনোই অন্য শিশুদের সাথে তুলনা করবেন না। তাদের শক্তিগুলোকে চিনুন এবং সেই দিকে এগিয়ে যেতে তাদের উৎসাহিত করুন।`,
		},
		{
			Title:    "সন্তানের সাথে কার্যকর যোগাযোগ: শিশু কেন কথা শোনে না?",
			Category: "teen_parenting",
			ImageURL: "https://images.unsplash.com/photo-1609234656388-40f8821beb6b?w=800",
			Content: `শিশু বিশেষজ্ঞদের মতে, শিশুরা অবাধ্য হয়ে কথা শোনে না — ব্যাপারটা এমন নয়। অনেক সময় সমস্যা হয় আমাদের কথা বলার ধরনে। কথা বলার ধরনে একটু পরিবর্তন আনলেই শিশুরা অনেক বেশি মনোযোগী হতে পারে।

১. একসঙ্গে অনেক কথা বলবেন না — এতগুলো নির্দেশ একসঙ্গে দিলে ছোট শিশুর পক্ষে সব মনে রাখা কঠিন। কাজগুলো ভাগ করে বলুন। প্রথমে বলুন, 'কার্টুন শেষ হলে টিভি বন্ধ করো।' সেটা হয়ে গেলে বলুন, 'এখন দাঁত ব্রাশ করো।'

২. সরাসরি বলুন — 'পড়তে বসবেন' না করে বলুন, 'আমরা পার্কে যাচ্ছি। এখন জুতা পরে নাও।' ছোট ও পরিষ্কার করে কথা বললে শিশুদের শুনতে, বুঝতে ও মানতে সুবিধা হয়।

৩. দূর থেকে চিৎকার না করে কাছে যান — রান্নাঘর থেকে চেঁচিয়ে বললেন, 'পড়তে বসো!' কিন্তু শিশুর মন তখন খেলায়। সে হয়তো শুনলই না। তাই কাছে গিয়ে, চোখের দিকে তাকিয়ে কথা বলুন। প্রয়োজনে কাঁধে আলতো করে হাত রাখুন।

৪. প্রতিফলিত শ্রবণ — আপনার সন্তানকে দেখানোর একটি দুর্দান্ত উপায় হলো, আপনি মনোযোগ দিচ্ছেন এবং তাদের যা বলছে তার প্রতি আপনি যত্নবান — সেটি আয়নার মতো করে তাদেরকে বুঝানো। তারা আপনাকে যা বলে, বিভিন্ন শব্দ ব্যবহার করে তার পুনরাবৃত্তি করুন।

৫. বয়সোপযোগী ভাষা ব্যবহার করুন — আপনার সন্তানের জন্য বোধগম্য ও তার বয়স উপযোগী ভাষা ব্যবহার করুন। স্পষ্ট ও সুনির্দিষ্ট শব্দ ব্যবহার করুন। অবমাননাকর শব্দ ব্যবহার করবেন না।

৬. নিজে উদাহরণ হোয়েন — সন্তানের কাছে বাবা-মাই পৃথিবীতে প্রথম আত্মীয়। আপনার সন্তান আপনাকে যা করতে দেখেছে তা-ই গুরুত্বপূর্ণ। সন্তানকে শুধু এমন প্রতিশ্রুতিই দিন যেটা আপনি নিশ্চিতভাবে বাস্তবায়ন করতে পারবেন।

৭. সন্তানের কথাও মন দিয়ে শুনুন — সন্তান যখন কোনো গল্প বলছে, স্কুলের কথা বলছে বা নতুন কিছু দেখাতে চাইছে, তখন একটু সময় দিন। মোবাইলটা হাত থেকে নামিয়ে রাখুন। তার দিকে তাকান। প্রশ্ন করুন। শিশুরা যখন বুঝতে পারে, তাদের কথা গুরুত্ব দিয়ে শোনা হচ্ছে, তখন তারাও অন্যের কথা বেশি মনোযোগ দিয়ে শুনতে শেখে।

সন্তানকে কথা শোনানোর সবচেয়ে ভালো উপায় হলো—কম কথা বলা, স্পষ্ট করে বলা, ধৈর্য ধরা এবং তার কথাও মন দিয়ে শোনা।`,
		},
		{
			Title:    "শিশুর পুষ্টি: বয়স অনুযায়ী সুষম খাবার গাইড",
			Category: "child-development",
			ImageURL: "https://images.unsplash.com/photo-1490818387583-1baba5e638af?w=800",
			Content: `শিশুর সঠিক বিকাশের জন্য পুষ্টিকর খাবার অত্যন্ত গুরুত্বপূর্ণ। জন্মের পর প্রথম কয়েক বছর শিশুর শারীরিক ও মানসিক বিকাশ অত্যন্ত দ্রুত ঘটে। তাই পিতা-মাতার উচিত শিশুকে পর্যাপ্ত পুষ্টিকর খাবার প্রদান করা।

০-৬ মাস: শুধু মায়ের বুকের দুধ — নবজাতকের জন্য মায়ের বুকের দুধ সবচেয়ে উপকারী। এতে শিশুর প্রয়োজনীয় সকল পুষ্টি উপাদান রয়েছে। জন্মের ১ ঘণ্টার মধ্যে বুকের দুধ খাওয়ানো শুরু করুন। প্রতি ২-৩ ঘণ্টা পর পর খাওয়ান।

৬-১২ মাস: পরিপূরক খাবার শুরু — বুকের দুধের পাশাপাশি পরিপূরক খাবার শুরু করতে হবে। প্রথমে নরম ও সহজে হজমযোগ্য খাবার যেমন সবজি ও ফলের পিউরি, সুজি, খিচুড়ি ইত্যাদি দিতে হবে। ধীরে ধীরে খাবারের পরিমাণ ও ঘনত্ব বাড়াতে হবে।

১-৩ বছর: সুষম খাদ্য — শিশুকে সুষম খাবার দিতে হবে। খাবারে শস্য, ডাল, সবজি, ফল, মাছ, মাংস, ডিম এবং দুধ ও দুগ্ধজাত দ্রব্য অন্তর্ভুক্ত করতে হবে। জাঙ্ক ফুড ও চিনি যুক্ত খাবার এড়িয়ে চলতে হবে।

৪-১২ বছর: স্কুল-বয়েসী পুষ্টি — সকালে স্বাস্থ্যকর নাস্তা দেওয়া খুব জরুরি। বিদ্যালয়ের টিফিনে স্বাস্থ্যকর খাবার দিতে উৎসাহিত করুন। প্রচুর পানি পান করান।

মূল পুষ্টি নিয়ম:

• প্রতিদিন ফল ও সবজি দিন
• প্রতি খাবারে আনাজ বা রুটি থাকুক
• ডাল, মাছ, মাংস, ডিম প্রতিদিন খাওয়ান
• দুধ ও দুগ্ধজাত পণ্য দিন
• চিনি ও তেলযুক্ত খাবার কমান
• প্রতিদিন পর্যাপ্ত পানি পান করান

মনে রাখবেন, প্রতিটি শিশু ভিন্ন। তাদের খাদ্যাভ্যাসও ভিন্ন। ধৈর্য ধরে বিভিন্ন ধরনের খাবার পরীক্ষা করান। জোর করে খাওয়ানো বন্ধ করুন। শিশু যখন ক্ষুধার্ত হবে, তখনই খাবে।`,
		},
		{
			Title:    "শিশুর ঘুম: বয়স অনুযায়ী সঠিক ঘুমের নিয়ম ও টিপস",
			Category: "mental_health",
			ImageURL: "https://images.unsplash.com/photo-1519689680058-324335c77eba?w=800",
			Content: `পর্যাপ্ত ও গুণগত ঘুম শিশুর শারীরিক ও মানসিক বিকাশের জন্য অপরিহার্য। ঘুমের অভাবে শিশুরা ঝগড়াঝাঁটি করে, মনোযোগ কমে যায় এবং পড়াশোনায় মন লাগে না।

বয়স অনুযায়ী ঘুমের প্রয়োজন:

• নবজাতক (০-৩ মাস): দিনে ১৪-১৭ ঘণ্টা। কোনো নির্দিষ্ট সময়সূচি নেই।
• শিশু (৪-১১ মাস): দিনে ১২-১৫ ঘণ্টা। রাতের ঘুম ও দিনের ঘুমে ভাগ থাকবে।
• টোডলার (১-২ বছর): দিনে ১১-১৪ ঘণ্টা।
• প্রি-স্কুলার (৩-৫ বছর): দিনে ১০-১৩ ঘণ্টা।
• স্কুল-বয়ী (৬-১২ বছর): দিনে ৯-১২ ঘণ্টা।
• কিশোর (১৩-১৮ বছর): দিনে ৮-১০ ঘণ্টা।

ঘুমের টিপস:

১. নিয়মিত ঘুমের সময়সূচি — প্রতিদিন একই সময়ে ঘুমানোর অভ্যাস গড়ে তুলুন। সপ্তাহের দিন ও ছুটির দিনেও একই সময়ে ঘুমান।

২. ঘুমের রুটিন তৈরি করুন — ঘুমানোর ৩০ মিনিট আগে থেকে শান্ত হয়ে যান। বই পড়ুন, গান শুনুন, গল্প বলুন। স্ক্রিন থেকে দূরে থাকুন।

৩. নিরাপদ ঘুমের পরিবেশ — শান্ত ও অন্ধকার পরিবেশ তৈরি করুন। তাপমাত্রা স্বাভাবিক রাখুন। অতিরিক্ত বালিশ বা কম্বল ব্যবহার করবেন না।

৪. স্ক্রিন টাইম বন্ধ করুন — ঘুমানোর অন্তত ১ ঘণ্টা আগে মোবাইল, টিভি, ট্যাবলেট সরিয়ে রাখুন। নীল আলো ঘুমের হরমোনকে বাধা দেয়।

৫. শিশুকে চিৎ হয়ে শোয়ান — নবজাতকদের সবসময় পিঠের উপর চিৎ করে শুইয়ে রাখুন। এটি SIDS (Sudden Infant Death Syndrome) রোধে সাহায্য করে।

৬. খাওয়ার পর ঘুমাতে দিন — শিশু যখন ক্ষুধার্ত নেই এবং স্বস্তিতে আছে, তখন ঘুমাতে দিন।

৭. দিনের ঘুম কমানো — ৫ বছরের পর দিনের ঘুম ধীরে ধীরে কমান। দীর্ঘ দিনের ঘুম রাতের ঘুমে বাধা দিতে পারে।

৮. ঘুম ঘাটলে কী হয় — ঘুমের অভাবে শিশুরা ঝগড়াঝাঁটি করে, রাগ করে, মনোযোগ কমে যায়, খাওয়ার আগ্রহ কমে যায়, রোগ প্রতিরোধ ক্ষমতা দুর্বল হয়।

মনে রাখবেন, প্রতিটি শিশু ভিন্ন। কিছু শিশু বেশি ঘুমায়, কিছু কম। শিশুর আচরণ লক্ষ্য করুন। যদি শিশু দিনে সচল থাকে, ভালো খায় এবং রাগ কম করে — তাহলে তার পর্যাপ্ত ঘুম হচ্ছে।`,
		},
		{
			Title:    "শিশুর খেলাধুলা ও শারীরিক বিকাশ: সামগ্রিক উন্নয়নের পথ",
			Category: "child-development",
			ImageURL: "https://images.unsplash.com/photo-1472162072942-cd5147eb3902?w=800",
			Content: `শিশুর জীবনে খেলাধুলা ও শরীরচর্চা যুক্ত করা মানে তার ভবিষ্যতের জন্য বিনিয়োগ করা। এটি তাদের সুস্থ শরীর, শক্তিশালী মন এবং আজীবনের জন্য প্রয়োজনীয় মূল্যবোধ গড়ে দেয়।

খেলাধুলার উপকারিতা:

১. শারীরিক বিকাশ — অঙ্গ-প্রত্যঙ্গ নাড়া-চাড়ার ফলে ব্যায়াম হয় এবং অঙ্গপ্রত্যঙ্গ সুগঠিত হয়। রক্ত চলাচল ভালো হয়। মাংস পেশি সতেজ ও সবল থাকে।

২. মানসিক বিকাশ — খেলাধুলায় যুক্ত শিশুদের মনোযোগ ও স্মৃতিশক্তি বৃদ্ধি পায়। তুলনামূলক কম মানসিক চাপ থাকে।

৩. পড়াশোনায় সুবিধা — খেলাধুলায় যুক্ত শিশুরা সময়ের সঠিক ব্যবহার ও শৃঙ্খলা শেখে।

৪. মানসিক স্বাস্থ্য — খেলাধুলায় যুক্ত শিশুরা বেশি আত্মবিশ্বাসী ও আবেগ নিয়ন্ত্রণে দক্ষ হয়।

৫. ঘুম — খেলাধুলায় যুক্ত শিশুরা গভীর ও আরামদায়ক ঘুম পায়।

৬. খাদ্যাভ্যাস — খেলোয়াড় শিশুরা সাধারণত স্বাস্থ্যকর খাবারের প্রতি আগ্রহী হয়।

পারিবারিক খেলাধুলার আইডিয়া:

• ছুটির দিনে প্রকৃতির মাঝে হাঁটা বা ট্রেকিং
• বিকেলে সবাই মিলে সাইকেল চালানো
• ব্যাডমিন্টন — বাংলাদেশের একটি জনপ্রিয় সংস্কৃতি
• পারিবারিক যোগব্যায়াম
• ছাদে বা উঠানে ক্রিকেট বা ফুটবল

খেলাধুলার পর কুল ডাউন — খেলাধুলার পর শরীরকে স্বাভাবিক অবস্থায় ফিরিয়ে আনা অত্যন্ত জরুরি। হালকা স্ট্রেচিং, গভীর শ্বাস-প্রশ্বাস নেওয়া এবং শান্ত হয়ে বসা পেশির ক্লান্তি দূর করতে সাহায্য করে।

নেলসন ম্যান্ডেলার বিখ্যাত উক্তি: "খেলাধুলার বিশ্বকে বদলে দেওয়ার ক্ষমতা আছে। এর অনুপ্রাণিত করার শক্তি আছে। এটি মানুষকে এমনভাবে ঐক্যবদ্ধ করতে পারে যা অন্য খুব কম জিনিসই পারে।"

মনে রাখবেন, পড়াশোনার পাশাপাশি খেলাধুলা শিশুর সুষম ও স্বাস্থ্যকর বেড়ে ওঠার জন্য অপরিহার্য।`,
		},
		{
			Title:    "অনলাইন নিরাপত্তা: সন্তানকে ডিজিটাল বিশ্বে নিরাপদ রাখার গাইড",
			Category: "screen_time",
			ImageURL: "https://images.unsplash.com/photo-1563986768609-322da13575f2?w=800",
			Content: `বর্তমানে শিশুরা তাদের জীবনের বেশ বড় একটা সময় ইন্টারনেটে কাটায়। UNICEF-এর ২০১৯ সালের সমীক্ষায় দেখা গেছে, দেশের সাইবার বুলিং বা সাইবার অপরাধের শিকারদের ১০ শতাংশ হচ্ছে ১৮ বছরের কম বয়সী শিশু।

শিশুদের শেখানো দরকার:

১. আসল নাম, স্কুল, ঠিকানা, ফোন নম্বর কাউকে দেবে না
২. অচেনা লোকের রিকোয়েস্ট অ্যাকসেপ্ট করবে না
৩. কেউ ন্যুড চাইলে, ভয় দেখালে স্ক্রিনশট নিয়ে বাবা-মাকে বলবে
৪. সাইবার বুলিং হলে ব্লক করবে, রিপোর্ট করবে
৫. বিপজ্জনক গেমে ক্লিক করবে না

অভিভাবকদের করণীয়:

• ফোনে প্যারেন্টাল কন্ট্রোল অন রাখুন
• রাতে ফোন নিজের কাছে রাখুন
• সপ্তাহে একদিন সন্তানের সাথে বসে ফোন ঘাঁটুন
• বিশ্বাস রাখুন, কিন্তু অন্ধ বিশ্বাস না
• মোবাইল দিয়েই দায়িত্ব শেষ না — কথা বলুন, বোঝান

ইউনিসেফ বাংলাদেশের তথ্য: ২৫ লাখের বেশি শিশু, বাবা-মা এবং শিক্ষকদের অনলাইন নিরাপত্তা বিষয়ে প্রশিক্ষিত করা হয়েছে। শিশুদের নিরাপদে রাখতে পারিবারিক আলোচনা জরুরি।

মনে রাখবেন, শিশুদের ইন্টারনেট থেকে বাদ দেওয়া সমাধান নয়। বরং তাদের সঠিক নির্দেশনা দেওয়াই আসল কাজ।`,
		},
		{
			Title:    "বয়ঃসন্ধি: কিশোর-কিশোরীদের শরীরে পরিবর্তন ও অভিভাবকের ভূমিকা",
			Category: "teen_parenting",
			ImageURL: "https://images.unsplash.com/photo-1529333166437-7750a6dd5a70?w=800",
			Content: `সাধারণত ১০ থেকে ১৯ বছর বয়স পর্যন্ত সময়টিকে কৈশোর বা বয়ঃসন্ধিকাল বলে। এই বয়সে ছেলে মেয়েদের বিভিন্ন শারীরিক ও মানসিক পরিবর্তন দেখা দেয়।

সাধারণ শারীরিক পরিবর্তন (উভয়ের ক্ষেত্রে):

• দ্রুত শারীরিক বৃদ্ধি — উচ্চতা ও ওজন খুব দ্রুত বৃদ্ধি পায়
• শরীরের গড়ন — হাড় ও পেশির ঘনত্ব বাড়তে শুরু করে
• লোম গজানো — বগল এবং জননাঙ্গের চারপাশে লোম গজাতে শুরু করে
• ত্বকের পরিবর্তন — ত্বক তৈলাক্ত হয়ে যায় এবং ব্রন দেখা দিতে পারে
• ঘাম — ঘর্মগ্রন্থিগুলো বেশি সক্রিয় হয়ে ওঠে

ছেলেদের বিশেষ পরিবর্তন:

• কণ্ঠস্বর ভারী হয়ে যায়
• মুখমণ্ডলে লোম গজায়
• পেশির পরিমাণ বাড়ে

মেয়েদের বিশেষ পরিবর্তন:

• ঋতুস্রাব শুরু হয় — এটি প্রজনন সক্ষমতা অর্জনের প্রধান লক্ষণ
• স্তনের বিকাশ ঘটে
• শরীরের আকৃতি পরিবর্তিত হয়

মানসিক পরিবর্তন:

• মন চঞ্চল হয়ে ওঠে
• দ্বিধা-দ্বন্দ্ব, আবেগ আর অস্থিরতা কাজ করে
• কেউ আবার একা থাকতে পছন্দ করে
• বন্ধুদের প্রতি বেশি নির্ভরশীল হয়

অভিভাবকদের করণীয়:

• খোলামেলা আলোচনা করুন — শরীরে কোনো অস্বস্তি বা অস্বাভাবিক কিছু মনে হলে কথা বলুন
• বয়ঃসন্ধিকালীন পরিবর্তন সম্পর্কে জানিয়ে দিন — এতে লজ্জা পাবার কিছু নেই
• ভালো বন্ধু নির্বাচনে সাহায্য করুন
• ধৈর্য ধরুন — এই পরিবর্তনগুলো সাময়িক`,
		},
		{
			Title:    "পরিবারের সঙ্গে সময় কাটানোর ১০টি কার্যকর উপায়",
			Category: "teen_parenting",
			ImageURL: "https://images.unsplash.com/photo-1511895426328-dc8714191300?w=800",
			Content: `আজকের ব্যস্ত জীবনে পরিবারের সঙ্গে সময় কাটানো কঠিন হতে পারে। কিন্তু শিশুর বিকাশের জন্য এটি অপরিহার্য। এখানে কিছু সহজ ও কার্যকর উপায় দেওয়া হলো:

১. পারিবারিক খাবার — রাতের খাবারের সময় সবাই একসাথে বসুন। ফোন, টিভি দূরে রাখুন। প্রত্যেকে তার দিনের কথা বলুন।

২. হাঁটা — সন্ধ্যায় পরিবারের সবাই মিলে হাঁটতে যান। এটি শারীরিক ও মানসিক উভয় স্বাস্থ্যের জন্য ভালো।

৩. রান্না — সন্তানদের সঙ্গে একসাথে রান্না করুন। এটি তাদের দক্ষতা বাড়ায় এবং মজার অভিজ্ঞতা।

৪. খেলাধুলা — বোর্ড গেম, কার্ড গেম, বা বাইরের খেলায় সবাই মিলে অংশ নিন।

৫. বই পড়া — পরিবারের সবাই মিলে বই পড়ুন বা গল্প শোনান।

৬. রবিবারের কার্যক্রম — ছুটির দিনে একসাথে বাইরে যান, সিনেমা দেখুন, বা বাগান করুন।

৭. সামাজিক কাজ — একসাথে প্রতিবেশীদের সাহায্য করুন বা সামাজিক কাজে অংশ নিন।

৮. সৃজনশীল কাজ — ছবি আঁকা, হস্তশিল্প, বা গান গাওয়া — এসব কাজে একসাথে সময় কাটান।

৯. ভ্রমণ — মাসে একবার পরিবারের সঙ্গে ভ্রমণে যান। এটি নতুন অভিজ্ঞতা তৈরি করে।

১০. আলিঙ্গন ও ভালোবাসা — প্রতিদিন সন্তানদের আলিঙ্গন করুন, ভালোবাসা জানান। এটি তাদের নিরাপদ বোধ করায়।

মনে রাখবেন, সময়ের পরিমাণ নয়, মান গুরুত্বপূর্ণ। দিনে ১৫-২০ মিনিট মনোযোগী সময়ও বড় পরিবর্তন আনতে পারে।`,
		},
		{
			Title:    "শিশুর আবেগ নিয়ন্ত্রণ: রাগ, হতাশা ও ভয় সামলানোর কৌশল",
			Category: "mental_health",
			ImageURL: "https://images.unsplash.com/photo-1494774157365-9e04c6720e47?w=800",
			Content: `শিশুরা তাদের আবেগ প্রকাশ করতে নানা রকম করে — মেজাজ খারাপ করে, কাঁদে, রাগ করে। এটি স্বাভাবিক। তবে তাদের আবেগ নিয়ন্ত্রণ শেখানো জরুরি।

আবেগ চিহ্নিত করুন — শিশু যখন রাগ করে বা কাঁদে, তখন তাকে বুঝিয়ে দিন, "তুমি এখন রাগান্বিত/দুঃখিত/ভীত।" এতে সে নিজের আবেগ চিনতে শেখে।

শান্ত হতে শেখান — গভীর শ্বাস নিতে বলুন। ধীরে ধীরে ৫ পর্যন্ত গুনতে বলুন। এটি স্নায়ুতন্ত্রকে শান্ত করে।

কথা বলুন — রাগ শান্ত হওয়ার পর কারেন সমস্যাটি নিয়ে কথা বলুন। তাকে বুঝিয়ে বলুন কেন সে রাগ করেছে এবং সঠিক পথ কী।

উদাহরণ দিন — আপনি যদি নিজে শান্তভাবে সমস্যার সমাধান করেন, তাহলে সন্তানও সেটা শিখবে।

সক্রিয় শ্রোতা হোয়েন — শিশু যখন কিছু বলতে চায়, তখন মনোযোগ দিয়ে শুনুন। তাকে বাধা দেবেন না।

পুরস্কার দিন — যখন শিশু নিজেকে নিয়ন্ত্রণ করতে পারে, তখন প্রশংসা করুন।

মনে রাখবেন, আবেগ নিয়ন্ত্রণ একদিনে শেখানো যায় না। ধৈর্য ধরে প্রতিদিন অনুশীলন করুন।`,
		},
		{
			Title:    "সৃজনশীলতা বিকাশ: শিশুর মধ্যে সৃজনশীল মনোভাব গড়ে তোলার উপায়",
			Category: "child-development",
			ImageURL: "https://images.unsplash.com/photo-1513364776144-60967b0f800f?w=800",
			Content: `প্রতিটি শিশুই জন্মগতভাবেই সৃজনশীল। তাদের এই সৃজনশীলতাকে ধরে রাখা ও বিকাশ করাই অভিভাবকের কাজ।

১. খেলার সুযোগ দিন — শিশুদের খেলতে দিন। খেলার সময় তারা নতুন কিছু তৈরি করে, সমস্যার সমাধান করে।

২. সামগ্রী দিন — রঙ, পেন্সিল, কাগজ, মাটি, কাঠের টুকরো — এসব জিনিস দিয়ে শিশু নতুন কিছু তৈরি করতে পারে।

৩. প্রশ্নের উত্তর দিন — শিশু যখন "কেন?", "কীভাবে?" জিজ্ঞেস করে, তখন ধৈর্য ধরে উত্তর দিন। এসব প্রশ্নই সৃজনশীলতার লক্ষণ।

৪. তুলনা করবেন না — প্রতিটি শিশুর সৃজনশীলতা ভিন্ন। অন্য শিশুর সাথে তুলনা করবেন না।

৫. স্ক্রিন টাইম কমান — অতিরিক্ত স্ক্রিন টাইম সৃজনশীলতার শত্রু। বই পড়া, ছবি আঁকা, খেলাধুলায় সময় কাটান।

৬. ভুল করতে দিন — শিশুকে ভুল করতে দিন। ভুল থেকেই সে শেখে। প্রতিবার সংশোধন করবেন না।

৭. স্বাধীনতা দিন — শিশুকে নিজে কিছু করতে দিন। সিদ্ধান্ত নিতে দিন।

৮. প্রকৃতির সঙ্গে যুক্ত করুন — প্রকৃতির মাঝে ঘুরতে নিন। পাখি, ফুল, পোকামাকড় দেখতে দিন। এতে তার কল্পনাশক্তি বাড়ে।

৯. গল্প বলুন — প্রতিদিন গল্প বলুন বা শোনান। গল্প শোনানো কল্পনাশক্তির সেরা ব্যায়াম।

১০. ধৈর্য ধরুন — সৃজনশীলতা রাতারাতি বৃদ্ধি পায় না। ধীরে ধীরে বৃদ্ধি পায়।`,
		},
		{
			Title:    "পরিবেশগত সচেতনতা: শিশুদের প্রকৃতির প্রতি ভালোবাসা শেখানোর উপায়",
			Category: "child-development",
			ImageURL: "https://images.unsplash.com/photo-1542601906990-b4d3fb778b09?w=800",
			Content: `শিশুদের প্রকৃতির প্রতি ভালোবাসা শেখানো তাদের পরিবেশগত সচেতনতা বাড়ায় এবং দায়িত্বশীল নাগরিক তৈরি করে।

১. বাগান করুন — ছোট একটি বাগানে ফুল বা শাকসবজি লাগান। শিশুকে সেটি দেখাভরণ করতে দিন। মাটির সঙ্গে কাজ করলে সে প্রকৃতিকে বোঝে।

২. গাছের নাম শেখান — হাঁটতে গেলে বিভিন্ন গাছের নাম বলুন। পাখি, পোকামাকড় দেখতে দিন।

৩. রিসাইক্লিং শেখান — প্লাস্টিক, কাগজ, কাচ — এসব আলাদা করে ফেলতে শেখান। পুনর্ব্যবহারের গুরুত্ব বোঝান।

৪. পানি সাশ্রয় — পানি না নষ্ট করার কথা বলুন। হাত ধোয়ার পর ফোনেট বন্ধ করতে শেখান।

৫. বিদ্যুৎ সাশ্রয় — ঘর থেকে বের হলে আলো বন্ধ করতে শেখান।

৬. প্রকৃতির সৌন্দর্য — সুন্দর দৃশ্য দেখলে থেমে দেখতে বলুন। ছবি তুলতে পারেন।

৭. পশুপাখির যত্ন — প্রাণীদের সাথে ভালো আচরণ শেখান। পাখির খাবার রাখতে পারেন।

৮. পরিষ্কার-পরিচ্ছন্নতা — বাইরে গেলে আবর্জনা ফেলবেন না বলে শেখান।

৯. সবুজ ব্যবহার — পলিথিনের পরিবর্তে কাপড়ের ব্যাগ ব্যবহার করুন। শিশুও সেটা শিখবে।

১০. গল্প ও ভিডিও — প্রকৃতি সম্পর্কে গল্প বলুন, ভিডিও দেখান।

মনে রাখবেন, শিশুরা দেখে শেখে। আপনি যদি প্রকৃতিকে ভালোবাসেন, সন্তানও সেটা শিখবে।`,
		},
		{
			Title:    "দায়িত্বশীলতা শেখানো: শিশুকে দায়িত্ববোধ গড়ে তোলার সহজ উপায়",
			Category: "child-development",
			ImageURL: "https://images.unsplash.com/photo-1503454537195-1dcabb73ffb9?w=800",
			Content: `দায়িত্বশীলতা শিখানো শিশুর ব্যক্তিত্ব গঠনের অন্যতম গুরুত্বপূর্ণ অংশ। ছোটবেলা থেকেই দায়িত্ব দিলে শিশু বড় হয়ে দায়িত্বশীল মানুষ হয়।

বয়স অনুযায়ী দায়িত্ব:

২-৩ বছর: নিজের খেলনা গুছিয়ে রাখা, হাত-মুখ ধোয়া

৪-৫ বছর: নিজের ব্যাগ গোছানো, টেবিল থেকে প্লেট সরানো

৬-৮ বছর: নিজের ঘর পরিষ্কার করা, ছোট ভাই-বোনের যত্ন নেওয়া

৯-১২ বছর: বাসায় ছোটখাটো কাজ করা, স্কুলের কাজ নিজে করা

কীভাবে শেখাবেন:

১. ধাপে ধাপে শেখান — একসাথে সব কাজ দিলে শিশু ভয় পেয়ে যাবে। প্রতিটি কাজ আলাদাভাবে শেখান।

২. ধৈর্য ধরুন — প্রথমবার ভালো হবে না। ধীরে ধীরে শিখবে।

৩. প্রশংসা করুন — কাজ শেষ হলে প্রশংসা করুন। এতে সে উৎসাহিত হবে।

৪. নিজে উদাহরণ দিন — আপনি যদি নিজে দায়িত্বশীল হোন, সন্তানও শিখবে।

৫. ফলাফলের দায়িত্ব দিন — যদি স্কুলের কাজ না করে, তাহলে পরিণাম ভোগ করতে হবে। এটি শেখান।

৬. পছন্দের কাজ দিন — শিশুর পছন্দের কাজ দিলে সে বেশি আগ্রহ দেখাবে।

৭. সময় ঠিক করুন — প্রতিটি কাজের জন্য সময় ঠিক করুন।

৮. শেষ করতে শেখান — যে কাজ শুরু করেছে, তা শেষ করতে হবে।

মনে রাখবেন, দায়িত্বশীলতা একদিনে আসে না। প্রতিদিন একটু একটু করে শেখান।`,
		},
		{
			Title:    "ভাই-বোনের সম্পর্ক: শিশুদের মধ্যে সুন্দর সম্পর্ক গড়ে তোলার কৌশল",
			Category: "teen_parenting",
			ImageURL: "https://images.unsplash.com/photo-1476234251651-f353703a034d?w=800",
			Content: `ভাই-বোনের সম্পর্ক জীবনের সবচেয়ে দীর্ঘস্থায়ী সম্পর্ক। এই সম্পর্ককে সুন্দর করে গড়ে তোলা অভিভাবকের দায়িত্ব।

সমস্যার কারণ:

• মনোযোগের অভাব — এক সন্তান মনে করে অন্য সন্তান বেশি মনোযোগ পাচ্ছে
• তুলনা — "তুমি তোমার ভাইয়ের মতো পড়ো না"
• জায়গা ও খেলনা নিয়ে ঝগড়া
• বয়স ও সক্ষমতার পার্থক্য

সমাধানের উপায়:

১. তুলনা বন্ধ করুন — প্রতিটি শিশু অনন্য। তুলনা করবেন না।

২. পৃথক সময় দিন — প্রতিটি সন্তানের সাথে আলাদাভাবে সময় কাটান।

৩. সম্পর্ক শেখান — ভাই-বোনের মধ্যে ভাগাভাগি ও সহযোগিতা শেখান।

৪. ন্যায্য আচরণ — সবার সাথে একই রকম আচরণ করুন।

৫. একসাথে কাজ — রান্না, পরিষ্কার, খেলা — এসব একসাথে করতে বলুন।

৬. ঝগড়া মিটিয়ে দিন — ঝগড়া হলে উভয়কে শান্ত করুন। কার ভুল তা নির্ধারণ করুন।

৭. ভালোবাসা জানান — প্রত্যেককে আলাদাভাবে ভালোবাসা জানান।

৮. একে অপরকে সাহায্য করতে বলুন — বড় সন্তানকে ছোটের যত্ন নিতে বলুন।

৯. বন্ধুত্ব গড়ে তুলুন — ভাই-বোনদের বন্ধু হতে শেখান।

১০. ধৈর্য ধরুন — সম্পর্ক গড়ে তোলায় সময় লাগে।`,
		},
		{
			Title:    "বাবা-মায়ের ভূমিকা: শিশুর বিকাশে পিতা-মাতার গুরুত্বপূর্ণ অবদান",
			Category: "teen_parenting",
			ImageURL: "https://images.unsplash.com/photo-1536337005238-94b997371b40?w=800",
			Content: `শিশুর বিকাশে বাবা-মায়ের ভূমিকা সমান গুরুত্বপূর্ণ। প্রত্যেক অভিভাবকের জন্য কিছু মৌলিক করণীয় আছে।

বাবার ভূমিকা:

১. সক্রিয় অংশগ্রহণ — শিশুর খেলাধুলা, পড়াশোনা, দৈনন্দিন কাজে সক্রিয়ভাবে অংশ নিন।

২. বন্ধুত্বপূর্ণ সম্পর্ক — শিশুর সাথে বন্ধুর মতো আচরণ করুন। সে যখন কিছু বলতে চায়, তখন মনোযোগ দিয়ে শুনুন।

৩. উদাহরণ দিন — আপনি যা করেন, শিশু সেটা শেখে। ভালো আচরণ করুন।

৪. শ্রদ্ধা শেখান — শিশুকে বড়দের প্রতি শ্রদ্ধাশীল হতে শেখান।

৫. নিরাপত্তা দিন — শিশুকে অনুভব করান যে বাবা তার পাশে আছেন।

মায়ের ভূমিকা:

১. স্নেহ ও যত্ন — শিশুকে পর্যাপ্ত স্নেহ ও যত্ন দিন।

২. ধৈর্য — শিশুর সাথে ধৈর্য ধরে আচরণ করুন।

৩. শিক্ষা — শিশুকে নৈতিকতা, মূল্যবোধ ও জীবনের শিক্ষা দিন।

৪. সংযম — অতিরিক্ত শাসন থেকে বিরত থাকুন।

৫. আত্মবিশ্বাস — শিশুর মধ্যে আত্মবিশ্বাস জাগিয়ে তুলুন।

উভয়ের করণীয়:

• একে অপরকে শ্রদ্ধা করুন — শিশু সেটা শিখবে
• সিদ্ধান্তে একমত হোন — শিশুর সামনে মতবিরোধ দেখাবেন না
• পরস্পরকে সমর্থন দিন — শিশু দেখবে পরিবার কীভাবে একসাথে কাজ করে
• সন্তানের সঙ্গে সময় কাটান — প্রতিদিন অন্তত ২০ মিনিট
• সন্তানকে ভালোবাসা জানান — শরীরের ভাষায়, কথায়, কাজে`,
		},
		{
			Title:    "স্বাস্থ্যকর অভ্যাস: শিশুর জীবনে ভালো অভ্যাস গড়ে তোলার গাইড",
			Category: "child-development",
			ImageURL: "https://images.unsplash.com/photo-1498837167922-ddd27525d352?w=800",
			Content: `শৈশব থেকেই ভালো অভ্যাস গড়ে তুললে সেটি আজীবন সাথে থাকে। শিশুদের মধ্যে স্বাস্থ্যকর অভ্যাস তৈরি করার জন্য কিছু সহজ উপায়:

১. সকালের নাস্তা — প্রতিদিন সকালে স্বাস্থ্যকর নাস্তা খাওয়ান। ডিম, রুটি, দুধ, ফল — এসব দিন।

২. পানি পান — দিনে অন্তত ৬-৮ গ্লাস পানি পান করতে বলুন।

৩. ফল ও সবজি — প্রতিদিন ফল ও সবজি খাওয়ার অভ্যাস করুন।

৪. ঘুম — নির্দিষ্ট সময়ে ঘুমানোর অভ্যাস গড়ে তুলুন।

৫. হাত ধোয়া — খাওয়ার আগে ও পরে, বাথরুম থেকে বের হয়ে হাত ধুয়ে নিন।

৬. দাঁত পরিষ্কার — সকালে ও রাতে দাঁত ব্রাশ করুন।

৭. ব্যায়াম — প্রতিদিন অন্তত ৩০ মিনিট শারীরিক কার্যক্লাপ করুন।

৮. স্ক্রিন টাইম — দৈনিক ২ ঘণ্টার বেশি স্ক্রিন ব্যবহার করবেন না।

৯. আলোচনা — পরিবারের সঙ্গে একসাথে খাওয়া ও কথা বলা।

১০. ধৈর্য — অভ্যাস রাতারাতি গড়ে ওঠে না। ধীরে ধীরে গড়ে তুলুন।

কীভাবে শেখাবেন:

• নিজে উদাহরণ দিন — আপনি যা করেন, সন্তান সেটা শেখে
• খেলার মধ্যে শেখান — "আমরা খেলা খেলব, প্রথমে হাত ধুয়ে নিই"
• প্রশংসা করুন — ভালো কাজের প্রশংসা করুন
• ধারাবাহিক হোন — প্রতিদিন একই নিয়ম মেনে চলুন`,
		},
		{
			Title:    "শিশুর অধিকার: প্রতিটি শিশুর পাওয়ার অধিকার কী কী?",
			Category: "child-development",
			ImageURL: "https://images.unsplash.com/photo-1488521787991-ed7bbaae773c?w=800",
			Content: `প্রতিটি শিশুর কিছু মৌলিক অধিকার আছে যা অভিভাবকদের জানা ও বাস্তবায়ন করা দরকার।

১. জীবনের অধিকার — প্রতিটি শিশুর বেঁচে থাকার অধিকার আছে।

২. নিরাপত্তার অধিকার — শিশু যেন শারীরিক বা মানসিক নির্যাতন থেকে মুক্ত থাকে।

৩. শিক্ষার অধিকার — প্রতিটি শিশুর শিক্ষা গ্রহণের অধিকার আছে।

৪. স্বাস্থ্যের অধিকার — শিশুর স্বাস্থ্য সেবা পাওয়ার অধিকার আছে।

৫. খাওয়ার অধিকার — পর্যাপ্ত ও পুষ্টিকর খাবার পাওয়ার অধিকার।

৬. খেলার অধিকার — খেলতে ও বিনোদন পাওয়ার অধিকার।

৭. মতামত প্রকাশের অধিকার — শিশুর তার মতামত প্রকাশের অধিকার আছে।

৮. পরিবারের সঙ্গে থাকার অধিকার — শিশুর পরিবারের সঙ্গে থাকার অধিকার।

৯. সম্মানের অধিকার — শিশুকে সম্মান দেওয়া হোক।

১০. শোষণ থেকে মুক্তির অধিকার — শিশু যেন শোষণ থেকে মুক্ত থাকে।

অভিভাবকদের করণীয়:

• শিশুর অধিকার সম্পর্কে সচেতন হোন
• শিশুর অধিকার রক্ষা করুন
• শিশুকে তার অধিকার সম্পর্কে জানিয়ে দিন
• শিশুর মতামতকে গুরুত্ব দিন
• শিশুর নিরাপত্তা নিশ্চিত করুন

মনে রাখবেন, শিশুর অধিকার রক্ষা করাই তার সুস্থ বিকাশের ভিত্তি।`,
		},
		{
			Title:    "বাংলাদেশে শিক্ষা: সন্তানের শিক্ষায় অভিভাবকের ভূমিকা",
			Category: "study_habits",
			ImageURL: "https://images.unsplash.com/photo-1503676260728-1c00da094a0b?w=800",
			Content: `বাংলাদেশে শিক্ষা ব্যবস্থায় অভিভাবকদের ভূমিকা অত্যন্ত গুরুত্বপূর্ণ। সন্তানের শিক্ষায় সহায়তা করার জন্য কিছু কার্যকর উপায়:

১. পড়ার পরিবেশ তৈরি করুন — পরিষ্কার, শান্ত ও আলোকিত জায়গায় পড়ার ব্যবস্থা করুন।

২. সময়সূচি ঠিক করুন — প্রতিদিন একই সময়ে পড়ার অভ্যাস গড়ে তুলুন।

৩. স্কুলের সঙ্গে যোগাযোগ — শিক্ষকদের সঙ্গে নিয়মিত কথা বলুন।

৪. পড়াশোনায় সহায়তা — শিশু কষ্ট পেলে সাহায্য করুন। তবে সব কাজ নিজে করবেন না।

৫. উৎসাহ দিন — ভালো নম্বর পেলে প্রশংসা করুন। খারাপ নম্বর পেলে বকাঝকা নয়, উৎসাহ দিন।

৬. গ্রন্থাগার — শিশুকে গ্রন্থাগারে নিয়ে যান। বই পড়ার অভ্যাস গড়ে তুলুন।

৭. শিক্ষামূলক খেলা — শিক্ষামূলক খেলা ও অ্যাপ ব্যবহার করুন।

৮. সময়মতো খাওয়ান — পর্যাপ্ত খাবার ও ঘুম নিশ্চিত করুন।

৯. তুলনা করবেন না — অন্য শিশুর সাথে তুলনা করবেন না।

১০. ধৈর্য ধরুন — প্রতিটি শিশু ভিন্ন। তার গতিতে শিখতে দিন।

মনে রাখবেন, শিক্ষা শুধু পরীক্ষার নম্বর নয়। জ্ঞান অর্জনই আসল শিক্ষা।`,
		},
		{
			Title:    "শিশুর সামাজিক দক্ষতা: অন্যদের সাথে সুন্দর আচরণ শেখানোর উপায়",
			Category: "child-development",
			ImageURL: "https://images.unsplash.com/photo-1529156069898-49953e39b3ac?w=800",
			Content: `সামাজিক দক্ষতা শিশুর সম্পর্ক, কর্মক্ষমতা ও আত্মবিশ্বাসের ওপর সরাসরি প্রভাব ফেলে। শৈশব থেকেই সামাজিক দক্ষতা শেখানো জরুরি।

মূল সামাজিক দক্ষতা:

১. সালাম দেওয়া — বড়দের সালাম দিতে শেখান। ছোটদের সাথে সুন্দর আচরণ করতে শেখান।

২. ধন্যবাদ বলা — কেউ কিছু দিলে ধন্যবাদ বলতে শেখান।

৩. দুঃখিত বলা — ভুল হলে দুঃখিত বলতে শেখান।

৪. অনুমতি নেওয়া — অন্যের জিনিস নেওয়ার আগে অনুমতি নিতে শেখান।

৫. ভাগাভাগি — খেলনা, খাবার — সব ভাগ করে খেতে শেখান।

৬. শোনা — অন্য কথা বললে মনোযোগ দিয়ে শোনতে শেখান।

৭. সহানুভূতি — অন্য কষ্ট পেলে সহানুভূতি দেখাতে শেখান।

৮. সমস্যার সমাধান — ঝগড়া হলে কথা বলে সমাধান করতে শেখান।

কীভাবে শেখাবেন:

• নিজে উদাহরণ দিন — আপনি যা করেন, সেটাই শিশু শেখে
• খেলার মধ্যে শেখান — ভূমিকা পালন করে শেখান
• গল্প বলুন — সামাজিক দক্ষতা সম্পর্কে গল্প বলুন
• প্রশংসা করুন — ভালো আচরণের প্রশংসা করুন
• ধৈর্য ধরুন — সামাজিক দক্ষতা ধীরে ধীরে শেখে`,
		},
		{
			Title:    "ক্ষুধা ও পুষ্টি: শিশুর সঠিক খাদ্যাভ্যাস গড়ে তোলার কৌশল",
			Category: "child-development",
			ImageURL: "https://images.unsplash.com/photo-1490818387583-1baba5e638af?w=800",
			Content: `শিশুর সঠিক খাদ্যাভ্যাস গড়ে তোলা অনেক সময় চ্যালেঞ্জিং হতে পারে। তবে কিছু কৌশল অবলম্বন করলে এটি সহজ হয়ে যায়।

শিশুর খাদ্যাভ্যাসের সমস্যা:

• খেতে চায় না
• নির্দিষ্ট খাবার ছাড়ে না
• জাঙ্ক ফুড খেতে চায়
• খাওয়ার সময় খেলতে চায়

সমাধানের উপায়:

১. নির্দিষ্ট সময়ে খাওয়ান — প্রতিদিন একই সময়ে খাওয়ান। ক্ষুধা লাগবে।

২. খাবার আকর্ষণীয় করুন — রঙিন খাবার, সুন্দর উপস্থাপনা।

৩. একসাথে খান — পরিবারের সবাই একসাথে বসে খান।

৪. জোর করবেন না — জোর করলে শিশু আরো বিদ্রোহী হয়।

৫. বিকল্প দিন — যদি একটি খাবার না খায়, তাহলে অন্য খাবার দিন।

৬. স্ন্যাক্স কমান — খাওয়ার আগে স্ন্যাক্স খাওয়া বন্ধ করুন।

৭. হলুদ খাবার — সবুজ শাকসবজি হলুদ করে দিন।

৮. রান্নায় সাহায্য — শিশুকে রান্নায় সাহায্য করতে দিন।

৯. ধৈর্য ধরুন — একটি খাবার ১০-১৫ বার উপস্থাপন করুন।

১০. পুষ্টি সম্পর্কে শেখান — কোন খাবার কী কাজ করে, তা বুঝিয়ে দিন।`,
		},
		{
			Title:    "ঘুমের সমস্যা: শিশু ঘুমাতে না চাইলে কী করবেন?",
			Category: "mental_health",
			ImageURL: "https://images.unsplash.com/photo-1519689680058-324335c77eba?w=800",
			Content: `অনেক শিশু ঘুমাতে চায় না। এটি একটি সাধারণ সমস্যা। তবে কিছু কৌশল অবলম্বন করলে সমস্যাটি সমাধান করা যায়।

ঘুমাতে না চাওয়ার কারণ:

• ভয় — অন্ধকার, একা থাকার ভয়
• উত্তেজনা — খেলাধুলা বা স্ক্রিনের পর মন শান্ত হয় না
• অভ্যাস — নির্দিষ্ট সময়ে ঘুমানোর অভ্যাস নেই
• শারীরিক সমস্যা — পেটে ব্যথা, জ্বর, দাঁতের ব্যথা

সমাধানের উপায়:

১. ঘুমের রুটিন — প্রতিদিন একই সময়ে ঘুমানোর অভ্যাস গড়ে তুলুন।

২. শান্ত পরিবেশ — ঘুমানোর আগে গল্প বলুন, গান শুনুন।

৩. স্ক্রিন বন্ধ — ঘুমানোর ১ ঘণ্টা আগে সব স্ক্রিন বন্ধ করুন।

৪. আলো কমান — ঘুমের সময় আলো কমিয়ে দিন।

৫. ভয় দূর করুন — অন্ধকারে ভয় পেলে হালকা আলো রাখুন।

৬. শারীরিক ক্লান্তি — দিনে পর্যাপ্ত খেলাধুলা করুন।

৭. খাওয়া — ঘুমানোর আগে হালকা খাবার দিন।

৮. আলিঙ্ঘন — ঘুমানোর আগে আলিঙ্ঘন করুন।

৯. গল্প — ঘুমানোর আগে গল্প বলুন।

১০. ধৈর্য — অভ্যাস রাতারাতি গড়ে ওঠে না। ধীরে ধীরে গড়ে তুলুন।

মনে রাখবেন, শিশু ঘুমাতে না চাইলে তাকে বকাঝকা করবেন না। ধৈর্য ধরে সমাধান খুঁজুন।`,
		},
		{
			Title:    "শিশুর সমস্যা: ঝগড়া, কান্না ও রাগ — কীভাবে মোকাবেলা করবেন",
			Category: "mental_health",
			ImageURL: "https://images.unsplash.com/photo-1544027993-37dbfe43562a?w=800",
			Content: `শিশুরা প্রায়শই ঝগড়া করে, কাঁদে, রাগ করে। এটি স্বাভাবিক। তবে এসব সমস্যার সমাধান করতে হবে।

ঝগড়া:

১. শান্ত থাকুন — ঝগড়া হলে আপনিও রাগ করবেন না।
২. উভয়কে শুনুন — কারের সমস্যা তা জেনে নিন।
৩. সমাধান খুঁজুন — কথা বলে সমাধান করতে শেখান।
৪. শাস্তি নয় — শাস্তির চেয়ে বোঝাপড়া ভালো।

কান্না:

১. কারেন সমস্যাটি জানুন — কেন কাঁদছে তা জেনে নিন।
২. আলিঙ্ঘন করুন — শান্ত করুন।
৩. কথা বলুন — সমস্যাটি নিয়ে কথা বলুন।
৪. সমাধান খুঁজুন — সমস্যার সমাধান খুঁজুন।

রাগ:

১. শান্ত হতে বলুন — গভীর শ্বাস নিতে বলুন।
২. কারেন রাগ তা জানুন — কেন রাগ করছে তা জেনে নিন।
৩. কথা বলুন — রাগ শান্ত হলে কথা বলুন।
৪. উদাহরণ দিন — আপনি যদি শান্তভাবে সমস্যা সমাধান করেন, সন্তানও শিখবে।

সাধারণ কৌশল:

• ধৈর্য ধরুন — শিশুরা ধীরে ধীরে শেখে
• উদাহরণ দিন — আপনার আচরণই শিশুর শিক্ষা
• প্রশংসা করুন — ভালো আচরণের প্রশংসা করুন
• সময় দিন — প্রতিদিন সন্তানের সঙ্গে সময় কাটান
• ভালোবাসা জানান — সন্তানকে ভালোবাসা জানান`,
		},
		{
			Title:    "ভালোবাসা প্রকাশ: সন্তানকে কীভাবে ভালোবাসা জানাবেন",
			Category: "teen_parenting",
			ImageURL: "https://images.unsplash.com/photo-1609234656388-40f8821beb6b?w=800",
			Content: `শিশুরা যখন ভালোবাসা পায়, তখন তারা নিরাপদ, আত্মবিশ্বাসী ও সুস্থ ব্যক্তিত্ব গড়ে তোলে। ভালোবাসা প্রকাশের নানা উপায়:

১. আলিঙ্ঘন — প্রতিদিন সন্তানকে আলিঙ্ঘন করুন। এটি সবচেয়ে সহজ ও কার্যকর উপায়।

২. কথায় বলুন — "আমি তোমাকে ভালোবাসি" — এই কথাটি প্রতিদিন বলুন।

৩. চোখের দিকে তাকান — কথা বলার সময় সন্তানের চোখের দিকে তাকান।

৪. সময় দিন — প্রতিদিন অন্তত ২০ মিনিট মনোযোগী সময় দিন।

৫. শোনুন — সন্তান যা বলছে তা মনোযোগ দিয়ে শুনুন।

৬. স্পর্শ — কাঁধে হাত রাখুন, মাথায় হাত বুলান।

৭. চিঠি লিখুন — ছোট একটি চিঠি লিখে দিন।

৮. উপহার — ছোট উপহার দিন।

৯. সহায়তা — সন্তান কষ্টে থাকলে পাশে থাকুন।

১০. গর্বিত — সন্তানের সাফল্যে গর্বিত হোন এবং বলুন।

ভালোবাসা প্রকাশের ভাষা:

• শরীরের ভাষা — আলিঙ্ঘন, হাসি, চোখের দিকে তাকানো
• কথার ভাষা — "তুমি আমার জন্য গুরুত্বপূর্ণ", "আমি তোমাকে ভালোবাসি"
• কাজের ভাষা — সময় দেওয়া, সাহায্য করা, শোনা

মনে রাখবেন, ভালোবাসা প্রকাশ করতে লজ্জা পাবেন না। সন্তানের জন্য এটি অত্যন্ত গুরুত্বপূর্ণ।`,
		},
	}

	for _, a := range articles {
		_, err := DB.Exec(ctx, `INSERT INTO articles (title, content, category, image_url, is_published, created_by) VALUES ($1, $2, $3, $4, TRUE, 1) ON CONFLICT DO NOTHING`, a.Title, a.Content, a.Category, a.ImageURL)
		if err != nil {
			log.Printf("Warning: failed to seed article '%s': %v", a.Title, err)
		}
	}
	fmt.Println("Parenting hub articles seeded")
}

func seedDemoNotes() {
	ctx := context.Background()

	var count int
	_ = DB.QueryRow(ctx, `SELECT COUNT(*) FROM notes`).Scan(&count)
	if count > 0 {
		return
	}

	notes := []struct {
		title      string
		content    string
		classLevel string
		subject    string
		chapter    string
		noteType   string
		language   string
	}{
		{"অঙ্ক - যোগ ও বিয়োগ", "যোগ (Addition): দুটি সংখ্যা একসাথে যোগ করা।\nউদাহরণ: ৫ + ৩ = ৮\n\nবিয়োগ (Subtraction): একটি সংখ্যা থেকে অন্য সংখ্যা বাদ দেওয়া।\nউদাহরণ: ১০ - ৪ = ৬\n\nঅনুশীলন:\n১) ৭ + ৫ = ১২\n২) ১২ - ৬ = ৬\n৩) ৮ + ৯ = ১৭\n৪) ১৫ - ৭ = ৮", "3", "Math", "যোগ ও বিয়োগ", "notes", "bn"},
		{"ইংরেজি - Alphabet", "A to Z alphabet.\n\nVowels: A, E, I, O, U\nConsonants: B, C, D, F, G, H, J, K, L, M, N, P, Q, R, S, T, V, W, X, Y, Z\n\nWords: Apple, Ant, Air, Ball, Cat, Dog, Elephant, Fish, Goat, Hat", "3", "English", "Alphabet", "notes", "en"},
		{"বাংলা - স্বরবর্ণ ও ব্যঞ্জনবর্ণ", "স্বরবর্ণ (১১টি): অ, আ, ই, ঈ, উ, ঊ, ঋ, এ, ঐ, ও, ঔ\n\nব্যঞ্জনবর্ণ (৩৫টি): ক, খ, গ, ঘ, ঙ, চ, ছ, জ, ঝ, ঞ, ট, ঠ, ড, ঢ, ণ, ত, থ, দ, ধ, ন, প, ফ, ব, ভ, ম, য, র, ল, শ, ষ, স, হ, ড়, ঢ়, য়\n\nঅনুশীলন: প্রতিটি ব্যঞ্জনবর্ণের সাথে স্বরবর্ণ যুক্ত করে লিখুন।", "3", "Bengali", "স্বরবর্ণ", "notes", "bn"},
		{"গণিত - গুণ ও ভাগ টেবিল", "গুণ টেবিল (২-১০):\n২ × ১ = ২   ২ × ২ = ৪   ২ × ৩ = ৬\n৩ × ১ = ৩   ৩ × ২ = ৬   ৩ × ৩ = ৯\n৪ × ১ = ৪   ৪ × ২ = ৮   ৪ × ৩ = ১২\n৫ × ১ = ৫   ৫ × ২ = ১০  ৫ × ৩ = ১৫\n\nভাগ: ২০ ÷ ৫ = ৪\nভাগ: ৩৬ ÷ ৬ = ৬\nভাগ: ৪৫ ÷ ৯ = ৫", "4", "Math", "গুণ ও ভাগ", "notes", "bn"},
		{"Science - Water Cycle", "The Water Cycle:\n\n1. Evaporation (বাষ্পীভবন) - Water turns to vapor from heat\n2. Condensation (ঘনীভবন) - Vapor turns back to water droplets\n3. Precipitation (বৃষ্টি) - Water falls as rain\n4. Collection (সংগ্রহ) - Water collects in rivers, lakes\n\nDiagram: Draw the cycle showing arrows between each stage.", "4", "Science", "Water Cycle", "notes", "en"},
		{"পদার্থবিজ্ঞান - বল ও গতি", "বল (Force): বস্তুর গতি পরিবর্তন করার ক্ষমতা।\n\nবলের প্রকারভেদ:\n১. অভিকর্ষ বল - পৃথিবী আমাদের টানে\n২. ঘর্ষণ বল - দুটি বস্তুর মধ্যে স্পর্শ করলে তৈরি\n৩. চুম্বকীয় বল - চুম্বক দ্বারা তৈরি\n\nগতি: বস্তুর অবস্থান পরিবর্তন।\nনিম্নলিখিত বস্তুগুলোর গতির উদাহরণ দিন।", "5", "Physics", "বল ও গতি", "notes", "bn"},
		{"রসায়ন - পদার্থের অবস্থা", "পদার্থের তিনটি অবস্থা:\n\n১. কঠিন (Solid): নির্দিষ্ট আকার ও আয়তন। অণু কাছাকাছি।\nউদাহরণ: পাথর, লোহা, বরফ\n\n২. তরল (Liquid): নির্দিষ্ট আয়তন কিন্তু আকার নেই।\nউদাহরণ: পানি, তেল, দুধ\n\n৩. গ্যাস (Gas): আয়তন ও আকার উভয়ই নেই।\nউদাহরণ: বাতাস, অক্সিজেন, জলীয়বাষ্প\n\nপরিবর্তন: বরফ → পানি → জলীয়বাষ্প", "6", "Chemistry", "পদার্থের অবস্থা", "notes", "bn"},
		{"Math - Fractions", "Fractions: Parts of a whole.\n\nNumerator / Denominator\n\n1/2 + 1/2 = 1\n1/4 + 2/4 = 3/4\n1/3 + 1/6 = 2/6 + 1/6 = 3/6 = 1/2\n\nPractice:\n1) 2/5 + 3/5 = ?\n2) 1/2 + 1/4 = ?\n3) 3/8 + 1/8 = ?", "6", "Math", "Fractions", "notes", "en"},
		{"জীববিজ্ঞান - কোষ", "কোষ হলো জীবনের সবচেয়ে ছোট একক।\n\nকোষের অংশ:\n১. কোষঝিল্লি - রক্ষা করে\n২. কোষদ্রব্য - তরল অংশ\n৩. নিউক্লিয়াস - নিয়ন্ত্রণ কেন্দ্র\n৪. মাইটোকন্ড্রিয়া - শক্তি তৈরি\n\nউদ্ভিজ্জ কোষ: গাছের পাতা, কাণ্ড\nপ্রাণী কোষ: রক্ত কোষ, স্নায়ু কোষ", "7", "Biology", "কোষ", "notes", "bn"},
		{"পদার্থবিজ্ঞান - তাপ ও তাপমাত্রা", "তাপমাত্রা মাপার একক:\n- সেলসিয়াস (°C)\n- ফারেনহাইট (°F)\n- কেলভিন (K)\n\nতাপ সঞ্চারের মাধ্যম:\n১. স্পর্শ (Conduction)\n২. স্রবণ (Convection)\n৩. বিকিরণ (Radiation)\n\nউদাহরণ: রেডিয়েটর দিয়ে ঘর গরম হয় - স্রবণ।", "8", "Physics", "তাপ ও তাপমাত্রা", "notes", "bn"},
		{"গণিত - বীজগণিত মৌলিক", "চলরাশি: x, y, z\nসমীকরণ: 2x + 5 = 15\n\nসমাধান:\n2x = 15 - 5\n2x = 10\nx = 5\n\nঅনুশীলন:\n১) 3x + 7 = 22 → x = 5\n২) 5x - 3 = 17 → x = 4\n৩) 2x + 4x = 24 → x = 4", "8", "Math", "বীজগণিত", "notes", "bn"},
		{"English - Tenses", "Three Main Tenses:\n\n1. Present Simple: I play football daily.\n2. Past Simple: I played football yesterday.\n3. Future Simple: I will play football tomorrow.\n\nPresent Continuous: I am playing football now.\nPast Continuous: I was playing football then.\n\nPresent Perfect: I have played football.\nPast Perfect: I had played football before he came.\n\nPractice: Convert to past tense:\n1) She writes a letter.\n2) They play cricket.", "8", "English", "Tenses", "notes", "en"},
	}

	for _, n := range notes {
		_, err := DB.Exec(ctx, `INSERT INTO notes (title, content, class_level, subject, chapter, type, language, is_published, created_by) VALUES ($1, $2, $3, $4, $5, $6, $7, TRUE, 1)`, n.title, n.content, n.classLevel, n.subject, n.chapter, n.noteType, n.language)
		if err != nil {
			log.Printf("Warning: failed to seed note '%s': %v", n.title, err)
		}
	}
	fmt.Println("Seeded 10 demo notes")
}

func seedDemoDailyContent() {
	ctx := context.Background()

	var count int
	_ = DB.QueryRow(ctx, `SELECT COUNT(*) FROM daily_content`).Scan(&count)
	if count > 0 {
		return
	}

	items := []struct {
		contentType string
		classLevel  string
		title       string
		body        string
		answer      string
		language    string
	}{
		{"vocabulary", "3", "শব্দ: সূর্য", "সূর্য = Sun\nউচ্চারণ: শূ-র্য\nবাক্য: সকালে সূর্য ওঠে।\nThe sun rises in the morning.", "", "bn"},
		{"vocabulary", "4", "Word: Beautiful", "Beautiful = সুন্দর\nPronunciation: BYOO-ti-ful\nSynonyms: Pretty, Lovely\nAntonyms: Ugly\nSentence: The flower is beautiful.", "", "en"},
		{"vocabulary", "5", "শব্দ: পরাবৃত্ত", "পরাবৃত্ত = Reflection\nআলোর কিরণ পৃষ্ঠ থেকে ফিরে যাওয়া।\nনিয়ম: পতন কোণ = পরাবৃত্ত কোণ\nউদাহরণ: আয়নায় মুখ দেখা।", "", "bn"},
		{"math", "3", "দ্রুত যোগ: ১০", "১০ + ৫ = ১৫\n১০ + ৮ = ১৮\n১০ + ১২ = ২২\nঅনুশীলন: ১০ + ৭ = ১৭, ১০ + ১৩ = ২৩", "", "bn"},
		{"math", "5", "ভগ্নাংশ যোগ", "১/৪ + ২/৪ = ৩/৪\n১/৩ + ১/৪ = ৪/১২ + ৩/১২ = ৭/১২\nLCM বের করুন, তারপর লব যোগ করুন।", "", "bn"},
		{"math", "7", "ত্রিকোণমিতি", "sin θ = লম্ব / অতিভুজ\ncos θ = ভূমি / অতিভুজ\ntan θ = লম্ব / ভূমি\nSOH-CAH-TOA মনে রাখুন।", "", "en"},
		{"science", "4", "আমাদের শরীর: হাড়", "মানুষের শরীরে ২০৬টি হাড়।\nকাজ: শরীরকে আকার দেয়, রক্ষা করে।\nফিমার - সবচেয়ে বড় হাড়।\nক্যালসিয়াম খান!", "", "bn"},
		{"science", "6", "তাপের প্রভাব", "তাপ সংকোচন: বস্তু ছোট হয়।\nতাপ প্রসারণ: বস্তু বড় হয়।\nগরম দিনে টায়ার ফেটে যায়।", "", "bn"},
		{"news", "5", "নতুন মেট্রোরেল", "ঢাকায় নতুন মেট্রোরেল চালু হয়েছে।\nগতি: ১০০ কিমি/ঘণ্টা\nপ্রতিদিন ৫ লক্ষ যাত্রী।", "", "bn"},
		{"news", "7", "নতুন গ্রহ আবিষ্কার", "Kepler-442b গ্রহ আবিষ্কৃত হয়েছে।\nদূরত্ব: ১,২০৬ আলোকবর্ষ।\nপৃথিবীর চেয়ে ১.৩ গুণ বড়।\nজীবন থাকতে পারে!", "", "en"},
	}

	for _, item := range items {
		_, err := DB.Exec(ctx, `INSERT INTO daily_content (content_type, class_level, title, body, answer, language, is_published) VALUES ($1, $2, $3, $4, $5, $6, TRUE)`, item.contentType, item.classLevel, item.title, item.body, item.answer, item.language)
		if err != nil {
			log.Printf("Warning: failed to seed daily content '%s': %v", item.title, err)
		}
	}
	fmt.Println("Seeded 10 demo daily content items")
}

func seedDemoTransitions() {
	ctx := context.Background()

	DB.Exec(ctx, `DELETE FROM student_transitions`)
	transitions := []struct {
		userID      int
		fromClass   int
		toClass     int
		gpa         float64
		resultNotes string
	}{
		{1, 3, 4, 4.2, "ভালো ফলাফল। গণিতে উজ্জ্বল পারফরম্যান্স।"},
		{1, 4, 5, 3.8, "মধ্যম ফলাফল। ইংরেজিতে আরও পরিশ্রম প্রয়োজন।"},
		{1, 5, 6, 5.0, "সর্বোচ্চ মার্কস! সকল বিষয়ে চমৎকার।"},
		{1, 6, 7, 4.5, "ভালো ফলাফল। বিজ্ঞানে বিশেষ দক্ষতা।"},
		{1, 7, 8, 4.0, "তৃতীয় বিভাগে উত্তীর্ণ। পদার্থবিজ্ঞানে ভালো।"},
		{1, 3, 4, 3.5, "চতুর্থ বিভাগে উত্তীর্ণ। পড়াশোনায় মনোযোগ দরকার।"},
		{1, 4, 5, 4.8, "প্রথম বিভাগে উত্তীর্ণ। সকল বিষয়ে চমৎকার।"},
		{1, 5, 6, 3.2, "পঞ্চম বিভাগে উত্তীর্ণ। গণিতে আরও অনুশীলন প্রয়োজন।"},
		{1, 6, 7, 4.7, "প্রথম বিভাগে উত্তীর্ণ। রসায়নে বিশেষ দক্ষতা।"},
		{1, 7, 8, 5.0, "সর্বোচ্চ মার্কস! সকল বিষয়ে অসাধারণ পারফরম্যান্স।"},
	}

	for _, t := range transitions {
		_, err := DB.Exec(ctx, `INSERT INTO student_transitions (user_id, from_class, to_class, gpa, result_notes, submitted_at) VALUES ($1, $2, $3, $4, $5, NOW())`, t.userID, t.fromClass, t.toClass, t.gpa, t.resultNotes)
		if err != nil {
			log.Printf("Warning: failed to seed transition for user %d: %v", t.userID, err)
		}
	}
	fmt.Println("Seeded 10 demo transitions")
}

func runNotificationsHistoryMigration() {
	ctx := context.Background()
	_, err := DB.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS notifications (
		id SERIAL PRIMARY KEY,
		title VARCHAR(255) NOT NULL,
		body TEXT NOT NULL,
		target VARCHAR(50) NOT NULL DEFAULT 'all',
		target_id INT DEFAULT 0,
		link_type VARCHAR(50) DEFAULT '',
		link_id INT DEFAULT 0,
		sent_by INT REFERENCES admin_users(id),
		sent_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`)
	if err != nil {
		log.Printf("Warning: failed to create notifications table: %v", err)
	}
	_, _ = DB.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_notifications_sent_at ON notifications(sent_at DESC)`)
	_, _ = DB.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_notifications_target ON notifications(target)`)

	// User-side read tracking
	_, err = DB.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS notification_reads (
		id SERIAL PRIMARY KEY,
		notification_id INT REFERENCES notifications(id) ON DELETE CASCADE,
		user_id INT REFERENCES users(id) ON DELETE CASCADE,
		read_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		UNIQUE(notification_id, user_id)
	)`)
	if err != nil {
		log.Printf("Warning: failed to create notification_reads table: %v", err)
	}
	_, _ = DB.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_notification_reads_user ON notification_reads(user_id)`)

	fmt.Println("Notifications history migration completed")
}

func runLiveExamMigration() {
	ctx := context.Background()
	alters := []string{
		`ALTER TABLE exams ADD COLUMN IF NOT EXISTS is_live BOOLEAN DEFAULT FALSE`,
		`ALTER TABLE exams ADD COLUMN IF NOT EXISTS total_marks INT DEFAULT 0`,
		`ALTER TABLE exams ADD COLUMN IF NOT EXISTS live_at TIMESTAMP WITH TIME ZONE DEFAULT NULL`,
		`ALTER TABLE exams ADD COLUMN IF NOT EXISTS class_level VARCHAR(20) DEFAULT ''`,
		`ALTER TABLE exams ADD COLUMN IF NOT EXISTS description TEXT DEFAULT ''`,
	}
	for _, q := range alters {
		_, _ = DB.Exec(ctx, q)
	}
	_, _ = DB.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_exams_live ON exams(is_live)`)
	fmt.Println("Live exam migration completed")
}

func runNotesMigration() {
	ctx := context.Background()
	_, err := DB.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS notes (
		id SERIAL PRIMARY KEY,
		class_level VARCHAR(20) NOT NULL,
		subject VARCHAR(255) NOT NULL,
		chapter VARCHAR(255) DEFAULT '',
		title VARCHAR(500) NOT NULL,
		content TEXT NOT NULL,
		type VARCHAR(50) DEFAULT 'notes',
		language VARCHAR(10) DEFAULT 'bn',
		tags TEXT[] DEFAULT '{}',
		is_published BOOLEAN DEFAULT TRUE,
		usage_count INT DEFAULT 0,
		created_by INT REFERENCES admin_users(id),
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`)
	if err != nil {
		log.Printf("Warning: failed to create notes table: %v", err)
	}
	_, _ = DB.Exec(ctx, `ALTER TABLE notes ADD COLUMN IF NOT EXISTS chapter VARCHAR(255) DEFAULT ''`)
	_, _ = DB.Exec(ctx, `ALTER TABLE notes ADD COLUMN IF NOT EXISTS type VARCHAR(50) DEFAULT 'notes'`)
	_, _ = DB.Exec(ctx, `ALTER TABLE notes ADD COLUMN IF NOT EXISTS language VARCHAR(10) DEFAULT 'bn'`)
	_, _ = DB.Exec(ctx, `ALTER TABLE notes ADD COLUMN IF NOT EXISTS usage_count INT DEFAULT 0`)
	_, _ = DB.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_notes_class ON notes(class_level)`)
	_, _ = DB.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_notes_subject ON notes(subject)`)
	fmt.Println("Notes migration completed")
}

func runDailyContentMigration() {
	ctx := context.Background()
	_, err := DB.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS daily_content (
		id SERIAL PRIMARY KEY,
		content_type VARCHAR(30) NOT NULL,
		class_level VARCHAR(20) NOT NULL,
		title VARCHAR(500) NOT NULL,
		body TEXT NOT NULL,
		answer TEXT DEFAULT '',
		language VARCHAR(10) DEFAULT 'bn',
		is_published BOOLEAN DEFAULT TRUE,
		created_by INT REFERENCES admin_users(id),
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`)
	if err != nil {
		log.Printf("Warning: failed to create daily_content table: %v", err)
	}
	_, _ = DB.Exec(ctx, `ALTER TABLE daily_content ADD COLUMN IF NOT EXISTS is_published BOOLEAN DEFAULT TRUE`)
	_, _ = DB.Exec(ctx, `ALTER TABLE daily_content DROP COLUMN IF EXISTS is_active`)
	_, _ = DB.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_daily_content_type ON daily_content(content_type)`)
	_, _ = DB.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_daily_content_class ON daily_content(class_level)`)
	_, _ = DB.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_daily_content_active ON daily_content(is_published)`)

	_, err = DB.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS daily_content_delivery (
		id SERIAL PRIMARY KEY,
		content_id INT REFERENCES daily_content(id) ON DELETE CASCADE,
		user_id INT REFERENCES users(id) ON DELETE CASCADE,
		delivered_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		viewed_at TIMESTAMP WITH TIME ZONE DEFAULT NULL,
		next_review_at TIMESTAMP WITH TIME ZONE DEFAULT NULL,
		review_count INT DEFAULT 0,
		UNIQUE(content_id, user_id)
	)`)
	if err != nil {
		log.Printf("Warning: failed to create daily_content_delivery table: %v", err)
	}
	_, _ = DB.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_delivery_user ON daily_content_delivery(user_id)`)
	_, _ = DB.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_delivery_review ON daily_content_delivery(next_review_at)`)
	fmt.Println("Daily content migration completed")
}

func runTransitionMigration() {
	ctx := context.Background()
	_, err := DB.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS student_transitions (
		id SERIAL PRIMARY KEY,
		user_id INT REFERENCES users(id) ON DELETE CASCADE,
		from_class INT NOT NULL,
		to_class INT NOT NULL,
		gpa DECIMAL(3,1) DEFAULT 0.0,
		subjects JSONB DEFAULT '[]',
		result_notes TEXT DEFAULT '',
		submitted_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`)
	if err != nil {
		log.Printf("Warning: failed to create student_transitions table: %v", err)
	}
	_, _ = DB.Exec(ctx, `ALTER TABLE student_transitions ALTER COLUMN from_class TYPE INT USING from_class::INT`)
	_, _ = DB.Exec(ctx, `ALTER TABLE student_transitions ALTER COLUMN to_class TYPE INT USING to_class::INT`)
	_, _ = DB.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_transitions_user ON student_transitions(user_id)`)

	_, err = DB.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS student_feedback (
		id SERIAL PRIMARY KEY,
		transition_id INT REFERENCES student_transitions(id) ON DELETE CASCADE,
		user_id INT REFERENCES users(id) ON DELETE CASCADE,
		feedback_type VARCHAR(30) NOT NULL,
		title VARCHAR(255) NOT NULL,
		message TEXT NOT NULL,
		guidelines JSONB DEFAULT '[]',
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`)
	if err != nil {
		log.Printf("Warning: failed to create student_feedback table: %v", err)
	}
	_, _ = DB.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_feedback_user ON student_feedback(user_id)`)
	fmt.Println("Transition migration completed")
}

// runStudentClassCleanupMigration normalizes users.student_class to the canonical "3".."8"
// range. Older rows may hold free text ("one", "Class 4") from before the class picker
// existed — digits embedded in the string (e.g. "Class 4") are extracted, anything left
// that still isn't a valid class is cleared to ” rather than guessed, since class-filtered
// features already treat ” as "no class set / show all" and a wrong guess would silently
// hide correct content instead.
func runStudentClassCleanupMigration() {
	ctx := context.Background()
	_, _ = DB.Exec(ctx, `UPDATE users SET student_class = regexp_replace(student_class, '\D', '', 'g') WHERE student_class ~ '\d'`)
	_, _ = DB.Exec(ctx, `UPDATE users SET student_class = '' WHERE student_class IS NOT NULL AND student_class NOT IN ('3','4','5','6','7','8')`)
	fmt.Println("Student class cleanup migration completed")
}

// runResultsMigration creates the table backing offline exam score entry —
// the direct data source for the guardian-facing progress/results dashboard.
// Unlike exam_results (MCQ live-exam scores), this is admin-entered by hand
// for paper-based offline tests, one row per (student, subject, exam).
func runResultsMigration() {
	ctx := context.Background()
	_, err := DB.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS student_results (
		id SERIAL PRIMARY KEY,
		user_id INT REFERENCES users(id) ON DELETE CASCADE,
		subject VARCHAR(255) NOT NULL,
		exam_name VARCHAR(255) NOT NULL,
		exam_date DATE NOT NULL,
		marks_obtained DECIMAL(6,2) NOT NULL,
		marks_total DECIMAL(6,2) NOT NULL,
		remarks TEXT DEFAULT '',
		created_by INT REFERENCES admin_users(id) ON DELETE SET NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`)
	if err != nil {
		log.Printf("Warning: failed to create student_results table: %v", err)
	}
	_, _ = DB.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_results_user ON student_results(user_id)`)
	_, _ = DB.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_results_subject ON student_results(subject)`)
	_, _ = DB.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_results_date ON student_results(exam_date)`)
	_, _ = DB.Exec(ctx, `ALTER TABLE student_results ADD COLUMN IF NOT EXISTS absent BOOLEAN NOT NULL DEFAULT false`)
	fmt.Println("Results migration completed")
}

// runPracticeMigration sets up interactive practice: vocabulary words (used
// for both flashcards and an auto-generated multiple-choice quiz),
// sentence-construction prompts, and an attempt log that both the streak
// calculation and the points/level model are derived from — nothing is
// pre-aggregated, it's all computed live from practice_attempts so a
// changed scoring rule doesn't require a backfill.
func runPracticeMigration() {
	ctx := context.Background()
	_, err := DB.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS vocabulary_words (
		id SERIAL PRIMARY KEY,
		class_level VARCHAR(20) NOT NULL,
		word VARCHAR(255) NOT NULL,
		meaning VARCHAR(255) NOT NULL,
		meaning_bn VARCHAR(255) DEFAULT '',
		example_sentence TEXT DEFAULT '',
		pronunciation VARCHAR(255) DEFAULT '',
		created_by INT REFERENCES admin_users(id) ON DELETE SET NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`)
	if err != nil {
		log.Printf("Warning: failed to create vocabulary_words table: %v", err)
	}
	_, _ = DB.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_vocab_class ON vocabulary_words(class_level)`)

	_, err = DB.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS sentence_exercises (
		id SERIAL PRIMARY KEY,
		class_level VARCHAR(20) NOT NULL,
		prompt VARCHAR(255) NOT NULL,
		sample_answer TEXT DEFAULT '',
		created_by INT REFERENCES admin_users(id) ON DELETE SET NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`)
	if err != nil {
		log.Printf("Warning: failed to create sentence_exercises table: %v", err)
	}
	_, _ = DB.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_sentence_class ON sentence_exercises(class_level)`)

	_, err = DB.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS practice_attempts (
		id SERIAL PRIMARY KEY,
		user_id INT REFERENCES users(id) ON DELETE CASCADE,
		activity_type VARCHAR(30) NOT NULL,
		item_id INT NOT NULL,
		is_correct BOOLEAN,
		points_earned INT DEFAULT 0,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`)
	if err != nil {
		log.Printf("Warning: failed to create practice_attempts table: %v", err)
	}
	_, _ = DB.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_practice_user ON practice_attempts(user_id)`)
	_, _ = DB.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_practice_user_date ON practice_attempts(user_id, created_at)`)

	fmt.Println("Practice migration completed")
}

func seedDemoVocabulary() {
	var count int
	_ = DB.QueryRow(context.Background(), `SELECT COUNT(*) FROM vocabulary_words`).Scan(&count)
	if count > 0 {
		return
	}

	ctx := context.Background()
	words := []struct {
		classLevel, word, meaning, meaningBn, example string
	}{
		{"3", "Happy", "Feeling joy", "সুখী", "She felt happy on her birthday."},
		{"3", "Big", "Large in size", "বড়", "The elephant is a big animal."},
		{"3", "Fast", "Moving quickly", "দ্রুত", "The cheetah is very fast."},
		{"3", "Quiet", "Making little or no noise", "শান্ত", "The library is a quiet place."},
		{"3", "Bright", "Full of light", "উজ্জ্বল", "The sun is very bright today."},
		{"4", "Brave", "Not afraid of danger", "সাহসী", "The brave firefighter saved the cat."},
		{"4", "Kind", "Caring and gentle", "দয়ালু", "It was kind of you to help."},
		{"4", "Curious", "Eager to learn", "কৌতূহলী", "The curious child asked many questions."},
		{"4", "Honest", "Truthful and sincere", "সৎ", "An honest person always tells the truth."},
		{"4", "Clever", "Quick to understand", "চালাক", "The clever fox escaped the trap."},
		{"5", "Ancient", "Very old, from long ago", "প্রাচীন", "They found an ancient temple."},
		{"5", "Generous", "Willing to give freely", "উদার", "She is generous with her time."},
		{"5", "Fragile", "Easily broken", "ভঙ্গুর", "Handle the glass vase, it is fragile."},
		{"5", "Cautious", "Careful to avoid danger", "সতর্ক", "Be cautious when crossing the road."},
		{"5", "Enormous", "Very large in size", "বিশাল", "The enormous whale surfaced near the boat."},
		{"6", "Diligent", "Hard-working and careful", "পরিশ্রমী", "The diligent student finished early."},
		{"6", "Reluctant", "Unwilling to do something", "অনিচ্ছুক", "He was reluctant to leave home."},
		{"6", "Vivid", "Producing strong, clear images", "স্পষ্ট", "She gave a vivid description of the trip."},
		{"6", "Humble", "Not proud or arrogant", "বিনয়ী", "Despite his success, he remained humble."},
		{"6", "Anxious", "Worried or nervous", "উদ্বিগ্ন", "She felt anxious before the exam."},
		{"7", "Ambitious", "Having a strong desire to succeed", "উচ্চাকাঙ্ক্ষী", "He is ambitious about his career."},
		{"7", "Genuine", "Truly what it is said to be", "প্রকৃত", "That is a genuine diamond."},
		{"7", "Resilient", "Able to recover quickly", "স্থিতিস্থাপক", "Children are often remarkably resilient."},
		{"7", "Reluctantly", "In an unwilling manner", "অনিচ্ছাসত্ত্বেও", "She reluctantly agreed to help."},
		{"7", "Optimistic", "Hopeful about the future", "আশাবাদী", "He stayed optimistic despite the setback."},
		{"8", "Meticulous", "Very careful and precise", "সূক্ষ্ম", "She is meticulous about her homework."},
		{"8", "Eloquent", "Fluent and persuasive in speech", "বাগ্মী", "The speaker was eloquent and confident."},
		{"8", "Inevitable", "Certain to happen", "অনিবার্য", "Change is inevitable as we grow."},
		{"8", "Versatile", "Able to adapt to many functions", "বহুমুখী", "She is a versatile player on the team."},
		{"8", "Profound", "Very great or intense", "গভীর", "The book had a profound effect on him."},
	}

	for _, w := range words {
		_, err := DB.Exec(ctx,
			`INSERT INTO vocabulary_words (class_level, word, meaning, meaning_bn, example_sentence) VALUES ($1,$2,$3,$4,$5)`,
			w.classLevel, w.word, w.meaning, w.meaningBn, w.example)
		if err != nil {
			log.Printf("Warning: failed to seed vocabulary word %q: %v", w.word, err)
		}
	}
	fmt.Printf("Seeded %d demo vocabulary words\n", len(words))
}

func seedDemoSentenceExercises() {
	var count int
	_ = DB.QueryRow(context.Background(), `SELECT COUNT(*) FROM sentence_exercises`).Scan(&count)
	if count > 0 {
		return
	}

	ctx := context.Background()
	exercises := []struct {
		classLevel, prompt, sample string
	}{
		{"3", "happy", "I am happy to see my friends."},
		{"3", "school", "I go to school every morning."},
		{"4", "brave", "The brave girl helped the lost puppy."},
		{"4", "friend", "My friend and I play cricket together."},
		{"5", "ancient", "We visited an ancient fort during vacation."},
		{"5", "generous", "The generous man donated books to the library."},
		{"6", "diligent", "The diligent worker finished the task on time."},
		{"6", "vivid", "The artist painted a vivid picture of the sunset."},
		{"7", "ambitious", "She has an ambitious plan to become a doctor."},
		{"7", "resilient", "The resilient team came back after losing the first match."},
		{"8", "meticulous", "The scientist kept meticulous records of the experiment."},
		{"8", "inevitable", "Growing up, mistakes are inevitable but valuable."},
	}

	for _, e := range exercises {
		_, err := DB.Exec(ctx,
			`INSERT INTO sentence_exercises (class_level, prompt, sample_answer) VALUES ($1,$2,$3)`,
			e.classLevel, e.prompt, e.sample)
		if err != nil {
			log.Printf("Warning: failed to seed sentence exercise %q: %v", e.prompt, err)
		}
	}
	fmt.Printf("Seeded %d demo sentence exercises\n", len(exercises))
}

func runOMRMigration() {
	ctx := context.Background()
	_, err := DB.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS omr_exams (
		id SERIAL PRIMARY KEY,
		title VARCHAR(255) NOT NULL,
		class_level VARCHAR(50) DEFAULT '',
		subject VARCHAR(100) DEFAULT '',
		question_count INT NOT NULL,
		columns INT NOT NULL DEFAULT 2,
		exam_code VARCHAR(10) NOT NULL UNIQUE,
		created_by INT REFERENCES admin_users(id) ON DELETE SET NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`)
	if err != nil {
		log.Printf("Warning: failed to create omr_exams table: %v", err)
	}

	_, err = DB.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS omr_questions (
		id SERIAL PRIMARY KEY,
		omr_exam_id INT REFERENCES omr_exams(id) ON DELETE CASCADE,
		question_number INT NOT NULL,
		correct_option INT NOT NULL CHECK (correct_option BETWEEN 1 AND 4),
		UNIQUE(omr_exam_id, question_number)
	)`)
	if err != nil {
		log.Printf("Warning: failed to create omr_questions table: %v", err)
	}

	_, err = DB.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS omr_students (
		id SERIAL PRIMARY KEY,
		omr_exam_id INT REFERENCES omr_exams(id) ON DELETE CASCADE,
		roll_number VARCHAR(10) NOT NULL,
		name VARCHAR(255) NOT NULL DEFAULT '',
		UNIQUE(omr_exam_id, roll_number)
	)`)
	if err != nil {
		log.Printf("Warning: failed to create omr_students table: %v", err)
	}

	_, err = DB.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS omr_sheets (
		id SERIAL PRIMARY KEY,
		omr_exam_id INT REFERENCES omr_exams(id) ON DELETE CASCADE,
		detected_roll_number VARCHAR(10) DEFAULT '',
		matched_student_id INT REFERENCES omr_students(id) ON DELETE SET NULL,
		status VARCHAR(20) NOT NULL DEFAULT 'needs_review',
		score INT DEFAULT 0,
		total_questions INT DEFAULT 0,
		raw_detection JSONB DEFAULT '{}',
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`)
	if err != nil {
		log.Printf("Warning: failed to create omr_sheets table: %v", err)
	}
	_, _ = DB.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_omr_sheets_exam ON omr_sheets(omr_exam_id)`)
	_, _ = DB.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_omr_questions_exam ON omr_questions(omr_exam_id)`)
	_, _ = DB.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_omr_students_exam ON omr_students(omr_exam_id)`)

	// enrollment_id links a roster row to a real enrollment (and, through it,
	// to a registered app login) so a scored sheet can be credited to an
	// actual student instead of just a typed-in roll/name pair.
	_, err = DB.Exec(ctx, `ALTER TABLE omr_students ADD COLUMN IF NOT EXISTS enrollment_id INT REFERENCES enrollments(id) ON DELETE SET NULL`)
	if err != nil {
		log.Printf("Warning: failed to add omr_students.enrollment_id: %v", err)
	}

	// student_result_id links a scored sheet to the student_results row it
	// produced, so a later correction updates that row instead of duplicating it.
	_, err = DB.Exec(ctx, `ALTER TABLE omr_sheets ADD COLUMN IF NOT EXISTS student_result_id INT REFERENCES student_results(id) ON DELETE SET NULL`)
	if err != nil {
		log.Printf("Warning: failed to add omr_sheets.student_result_id: %v", err)
	}

	// Sheet photos are never stored (scoring only persists the result), so
	// image_path — required on tables created before this — must stop being
	// mandatory. Existing rows/columns are left alone; new inserts just omit it.
	_, err = DB.Exec(ctx, `ALTER TABLE omr_sheets ALTER COLUMN image_path DROP NOT NULL`)
	if err != nil {
		log.Printf("Warning: failed to relax omr_sheets.image_path: %v", err)
	}

	fmt.Println("OMR migration completed")
}

func runPromoCodeMigration() {
	ctx := context.Background()
	_, err := DB.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS promo_codes (
		id SERIAL PRIMARY KEY,
		code VARCHAR(50) UNIQUE NOT NULL,
		discount_type VARCHAR(20) NOT NULL,
		discount_value INT NOT NULL,
		course_id INT REFERENCES courses(id) ON DELETE SET NULL,
		batch_id INT REFERENCES batches(id) ON DELETE SET NULL,
		max_redemptions INT,
		redemption_count INT NOT NULL DEFAULT 0,
		expires_at TIMESTAMP WITH TIME ZONE,
		is_active BOOLEAN NOT NULL DEFAULT TRUE,
		created_by INT REFERENCES admin_users(id),
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`)
	if err != nil {
		log.Printf("Warning: failed to create promo_codes table: %v", err)
	}

	_, err = DB.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS promo_code_redemptions (
		id SERIAL PRIMARY KEY,
		promo_code_id INT NOT NULL REFERENCES promo_codes(id) ON DELETE CASCADE,
		enrollment_id INT REFERENCES enrollments(id) ON DELETE SET NULL,
		mobile VARCHAR(20) NOT NULL,
		discount_amount INT NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`)
	if err != nil {
		log.Printf("Warning: failed to create promo_code_redemptions table: %v", err)
	}

	_, _ = DB.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_promo_redemptions_code ON promo_code_redemptions(promo_code_id)`)
	_, _ = DB.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_promo_redemptions_mobile ON promo_code_redemptions(mobile)`)
}

// runOMRDesignMigration adds the reusable "OMR design" (sheet layout: title,
// class, subject, question count, columns) that a token (an omr_exams row)
// is created from. A design has no answer key or roster of its own — those
// stay per-token, so the same design can be reused across several tokens
// (e.g. parallel exam sets) without cloning its layout by hand.
func runOMRDesignMigration() {
	ctx := context.Background()
	_, err := DB.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS omr_designs (
		id SERIAL PRIMARY KEY,
		title VARCHAR(255) NOT NULL,
		class_level VARCHAR(50) DEFAULT '',
		subject VARCHAR(100) DEFAULT '',
		question_count INT NOT NULL,
		columns INT NOT NULL DEFAULT 2,
		created_by INT REFERENCES admin_users(id) ON DELETE SET NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`)
	if err != nil {
		log.Printf("Warning: failed to create omr_designs table: %v", err)
	}

	// Each token (omr_exams row) now originates from a design instead of
	// carrying its own answer key at creation time.
	_, err = DB.Exec(ctx, `ALTER TABLE omr_exams ADD COLUMN IF NOT EXISTS omr_design_id INT REFERENCES omr_designs(id) ON DELETE SET NULL`)
	if err != nil {
		log.Printf("Warning: failed to add omr_exams.omr_design_id: %v", err)
	}

	// A token's question rows are now pre-created (one per question number,
	// correct_option unset) at token-creation time, then filled in later on
	// the token's detail page — so correct_option must allow NULL until then.
	_, err = DB.Exec(ctx, `ALTER TABLE omr_questions ALTER COLUMN correct_option DROP NOT NULL`)
	if err != nil {
		log.Printf("Warning: failed to relax omr_questions.correct_option: %v", err)
	}

	fmt.Println("OMR design migration completed")
}
