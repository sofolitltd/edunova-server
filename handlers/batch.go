package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"edunova-server/database"
	"edunova-server/models"
)

// buildScheduleText renders a human-readable routine string from structured
// days/times, e.g. ["Sat","Mon","Wed"] + "17:00"/"19:00" -> "Sat-Mon-Wed, 5:00 PM - 7:00 PM".
// Existing displays (course cards, enrollment views) read the plain
// `schedule` string, so this keeps them working without touching every
// call site — only batch creation/editing needs to know about the
// structured fields.
func buildScheduleText(days []string, startTime, endTime string) string {
	daysPart := strings.Join(days, "-")
	timePart := ""
	if startTime != "" && endTime != "" {
		timePart = formatTime(startTime) + " - " + formatTime(endTime)
	}
	switch {
	case daysPart != "" && timePart != "":
		return daysPart + ", " + timePart
	case daysPart != "":
		return daysPart
	default:
		return timePart
	}
}

// formatTime renders a 24h "HH:MM" as a 12h "H:MM AM/PM" string; anything
// that doesn't parse cleanly is returned unchanged.
func formatTime(t string) string {
	parts := strings.Split(t, ":")
	if len(parts) != 2 {
		return t
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return t
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return t
	}
	suffix := "AM"
	displayHour := hour
	if hour == 0 {
		displayHour = 12
	} else if hour == 12 {
		suffix = "PM"
	} else if hour > 12 {
		displayHour = hour - 12
		suffix = "PM"
	}
	return fmt.Sprintf("%d:%02d %s", displayHour, minute, suffix)
}

// nullableCourseID converts the zero value (no course selected) to nil so
// it's stored as SQL NULL instead of violating the courses FK with a
// nonexistent id=0.
func nullableCourseID(courseID int) *int {
	if courseID <= 0 {
		return nil
	}
	return &courseID
}

func batchNameExists(ctx context.Context, name string, excludeID int) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM batches WHERE LOWER(name) = LOWER($1)`
	args := []interface{}{name}
	if excludeID > 0 {
		query += ` AND id <> $2`
		args = append(args, excludeID)
	}
	query += `)`
	var exists bool
	err := database.DB.QueryRow(ctx, query, args...).Scan(&exists)
	return exists, err
}

func batchCodeExists(ctx context.Context, code string, excludeID int) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM batches WHERE code = $1`
	args := []interface{}{code}
	if excludeID > 0 {
		query += ` AND id <> $2`
		args = append(args, excludeID)
	}
	query += `)`
	var exists bool
	err := database.DB.QueryRow(ctx, query, args...).Scan(&exists)
	return exists, err
}

func studentIDExists(ctx context.Context, studentID string, excludeEnrollmentID int) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM enrollments WHERE student_id = $1`
	args := []interface{}{studentID}
	if excludeEnrollmentID > 0 {
		query += ` AND id <> $2`
		args = append(args, excludeEnrollmentID)
	}
	query += `)`
	var exists bool
	err := database.DB.QueryRow(ctx, query, args...).Scan(&exists)
	return exists, err
}

// nextStudentIDForBatch builds a suggested `<batch code><seq>` student ID,
// scanning existing enrollments under that batch's code prefix for the
// highest numeric suffix so IDs keep incrementing even after some are
// edited or removed.
func nextStudentIDForBatch(ctx context.Context, batchID int) (string, error) {
	var code string
	if err := database.DB.QueryRow(ctx, `SELECT COALESCE(code, '') FROM batches WHERE id = $1`, batchID).Scan(&code); err != nil {
		return "", err
	}
	rows, err := database.DB.Query(ctx, `SELECT student_id FROM enrollments WHERE student_id LIKE $1`, code+"%")
	if err != nil {
		return "", err
	}
	defer rows.Close()

	maxSeq := 0
	for rows.Next() {
		var sid string
		if err := rows.Scan(&sid); err != nil {
			continue
		}
		if n, err := strconv.Atoi(strings.TrimPrefix(sid, code)); err == nil && n > maxSeq {
			maxSeq = n
		}
	}
	return fmt.Sprintf("%s%04d", code, maxSeq+1), nil
}

func AdminNextStudentID(c *gin.Context) {
	batchID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid batch id"})
		return
	}
	studentID, err := nextStudentIDForBatch(c.Request.Context(), batchID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"student_id": studentID})
}

func AdminCheckStudentID(c *gin.Context) {
	studentID := strings.TrimSpace(c.Query("student_id"))
	if studentID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "student id is required"})
		return
	}
	excludeID, _ := strconv.Atoi(c.Query("id"))
	exists, err := studentIDExists(c.Request.Context(), studentID, excludeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"available": !exists})
}

func AdminGetBatches(c *gin.Context) {
	courseID := c.Query("course_id")
	classLevel := c.Query("class_level")
	shift := c.Query("shift")
	typeVal := c.Query("type")
	year := c.Query("year")
	ctx := context.Background()

	query := `SELECT b.id, b.class_level, b.course_id, COALESCE(c.title, ''), b.name, COALESCE(b.days, '{}'),
		COALESCE(b.start_time, ''), COALESCE(b.end_time, ''), COALESCE(b.schedule, ''),
		b.max_students, COALESCE(b.status, 'active'), b.admission_fee, b.note_fee, b.monthly_fee, b.shift, b.type, COALESCE(b.year, 0),
		COALESCE(b.section, ''), COALESCE(b.code, '')
		FROM batches b LEFT JOIN courses c ON b.course_id = c.id`
	var conditions []string
	var args []interface{}
	if courseID != "" {
		args = append(args, courseID)
		conditions = append(conditions, fmt.Sprintf("b.course_id = $%d", len(args)))
	}
	if classLevel != "" {
		args = append(args, classLevel)
		conditions = append(conditions, fmt.Sprintf("b.class_level = $%d", len(args)))
	}
	if shift != "" {
		args = append(args, shift)
		conditions = append(conditions, fmt.Sprintf("LOWER(b.shift) = LOWER($%d)", len(args)))
	}
	if typeVal != "" {
		args = append(args, typeVal)
		conditions = append(conditions, fmt.Sprintf("LOWER(b.type) = LOWER($%d)", len(args)))
	}
	if year != "" {
		args = append(args, year)
		conditions = append(conditions, fmt.Sprintf("b.year = $%d", len(args)))
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY b.class_level, b.name"

	rows, err := database.DB.Query(ctx, query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	var batches []models.Batch
	for rows.Next() {
		var b models.Batch
		if err := rows.Scan(&b.ID, &b.ClassLevel, &b.CourseID, &b.CourseName, &b.Name, &b.Days,
			&b.StartTime, &b.EndTime, &b.Schedule, &b.MaxStudents, &b.Status, &b.AdmissionFee, &b.NoteFee, &b.MonthlyFee, &b.Shift, &b.Type, &b.Year,
			&b.Section, &b.Code); err == nil {
			batches = append(batches, b)
		}
	}
	if batches == nil {
		batches = []models.Batch{}
	}
	c.JSON(http.StatusOK, batches)
}

func AdminCheckBatchName(c *gin.Context) {
	name := strings.TrimSpace(c.Query("name"))
	if name == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "batch name is required"})
		return
	}
	excludeID, _ := strconv.Atoi(c.Query("id"))
	exists, err := batchNameExists(c.Request.Context(), name, excludeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"available": !exists})
}

func AdminCheckBatchCode(c *gin.Context) {
	code := strings.TrimSpace(c.Query("code"))
	if code == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "batch code is required"})
		return
	}
	excludeID, _ := strconv.Atoi(c.Query("id"))
	exists, err := batchCodeExists(c.Request.Context(), code, excludeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"available": !exists})
}

func AdminCreateBatch(c *gin.Context) {
	var req models.CreateBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	if req.Status == "" {
		req.Status = "active"
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "batch name is required"})
		return
	}
	exists, err := batchNameExists(c.Request.Context(), req.Name, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, models.ErrorResponse{Error: "batch name already exists"})
		return
	}
	req.Code = strings.TrimSpace(req.Code)
	codeExists, err := batchCodeExists(c.Request.Context(), req.Code, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	if codeExists {
		c.JSON(http.StatusConflict, models.ErrorResponse{Error: "batch code already exists"})
		return
	}
	schedule := buildScheduleText(req.Days, req.StartTime, req.EndTime)
	courseID := req.CourseID

	var b models.Batch
	err = database.DB.QueryRow(context.Background(),
		`INSERT INTO batches (class_level, course_id, name, days, start_time, end_time, schedule, max_students, status, admission_fee, note_fee, monthly_fee, shift, type, year, section, code)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		 RETURNING id, class_level, course_id, COALESCE((SELECT title FROM courses WHERE id = $2), ''), name, COALESCE(days, '{}'),
		 COALESCE(start_time, ''), COALESCE(end_time, ''), COALESCE(schedule, ''), max_students, status, shift, type, COALESCE(year, 0), COALESCE(section, ''), COALESCE(code, ''), created_at, updated_at`,
		req.ClassLevel, courseID, req.Name, req.Days, req.StartTime, req.EndTime, schedule, req.MaxStudents, req.Status, req.AdmissionFee, req.NoteFee, req.MonthlyFee, req.Shift, req.Type, req.Year, req.Section, req.Code,
	).Scan(&b.ID, &b.ClassLevel, &b.CourseID, &b.CourseName, &b.Name, &b.Days, &b.StartTime, &b.EndTime,
		&b.Schedule, &b.MaxStudents, &b.Status, &b.Shift, &b.Type, &b.Year, &b.Section, &b.Code, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create batch"})
		return
	}
	c.JSON(http.StatusCreated, b)
}

func AdminUpdateBatch(c *gin.Context) {
	id := c.Param("id")
	idValue, err := strconv.Atoi(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid batch id"})
		return
	}

	var req models.CreateBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	if req.Status == "" {
		req.Status = "active"
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "batch name is required"})
		return
	}
	exists, err := batchNameExists(c.Request.Context(), req.Name, idValue)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, models.ErrorResponse{Error: "batch name already exists"})
		return
	}
	req.Code = strings.TrimSpace(req.Code)
	codeExists, err := batchCodeExists(c.Request.Context(), req.Code, idValue)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	if codeExists {
		c.JSON(http.StatusConflict, models.ErrorResponse{Error: "batch code already exists"})
		return
	}
	schedule := buildScheduleText(req.Days, req.StartTime, req.EndTime)
	courseID := req.CourseID

	var b models.Batch
	err = database.DB.QueryRow(context.Background(),
		`UPDATE batches SET class_level=$1, course_id=$2, name=$3, days=$4, start_time=$5, end_time=$6, schedule=$7, max_students=$8, status=$9, admission_fee=$10, note_fee=$11, monthly_fee=$12, shift=$13, type=$14, year=$16, section=$17, code=$18, updated_at=NOW()
		 WHERE id=$15
		 RETURNING id, class_level, course_id, COALESCE((SELECT title FROM courses WHERE id = $2), ''), name, COALESCE(days, '{}'),
		 COALESCE(start_time, ''), COALESCE(end_time, ''), COALESCE(schedule, ''), max_students, status, shift, type, COALESCE(year, 0), COALESCE(section, ''), COALESCE(code, ''), created_at, updated_at`,
		req.ClassLevel, courseID, req.Name, req.Days, req.StartTime, req.EndTime, schedule, req.MaxStudents, req.Status, req.AdmissionFee, req.NoteFee, req.MonthlyFee, req.Shift, req.Type, id, req.Year, req.Section, req.Code,
	).Scan(&b.ID, &b.ClassLevel, &b.CourseID, &b.CourseName, &b.Name, &b.Days, &b.StartTime, &b.EndTime,
		&b.Schedule, &b.MaxStudents, &b.Status, &b.Shift, &b.Type, &b.Year, &b.Section, &b.Code, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update batch"})
		return
	}
	c.JSON(http.StatusOK, b)
}

func AdminDeleteBatch(c *gin.Context) {
	id := c.Param("id")
	tag, err := database.DB.Exec(context.Background(), `DELETE FROM batches WHERE id = $1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "batch not found"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "batch deleted"})
}

// ========== BATCH <-> SUBJECT + SCHEDULE ==========

func AdminGetBatchSubjects(c *gin.Context) {
	batchID := c.Param("id")

	rows, err := database.DB.Query(context.Background(),
		`SELECT bs.id, bs.subject_id, s.name, bs.teacher_id, COALESCE(t.full_name, ''), COALESCE(bs.days, '{}'), COALESCE(bs.start_time, ''), COALESCE(bs.end_time, '')
		 FROM batch_subjects bs
		 JOIN subjects s ON bs.subject_id = s.id
		 LEFT JOIN teachers t ON bs.teacher_id = t.id
		 WHERE bs.batch_id = $1
		 ORDER BY s.name ASC`, batchID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	var subjects []models.BatchSubject
	for rows.Next() {
		var s models.BatchSubject
		if err := rows.Scan(&s.ID, &s.SubjectID, &s.SubjectName, &s.TeacherID, &s.TeacherName, &s.Days, &s.StartTime, &s.EndTime); err != nil {
			continue
		}
		subjects = append(subjects, s)
	}
	if subjects == nil {
		subjects = []models.BatchSubject{}
	}
	c.JSON(http.StatusOK, subjects)
}

// AdminAssignBatchSubject creates a new schedule entry for a subject in this
// batch. A subject can appear several times (e.g. taught at two different
// periods in the week), so this is always a plain insert, never an upsert.
func AdminAssignBatchSubject(c *gin.Context) {
	batchID := c.Param("id")

	var req models.AssignBatchSubjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	_, err := database.DB.Exec(context.Background(),
		`INSERT INTO batch_subjects (batch_id, subject_id, teacher_id, days, start_time, end_time) VALUES ($1, $2, $3, $4, $5, $6)`,
		batchID, req.SubjectID, req.TeacherID, req.Days, req.StartTime, req.EndTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to assign subject"})
		return
	}
	c.JSON(http.StatusCreated, models.SuccessResponse{Message: "subject assigned"})
}

// AdminUpdateBatchSubject edits one specific schedule entry (identified by
// its own row id, not by subject) — used when the admin edits a routine cell.
func AdminUpdateBatchSubject(c *gin.Context) {
	batchID := c.Param("id")
	entryID := c.Param("entryId")

	var req models.AssignBatchSubjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	tag, err := database.DB.Exec(context.Background(),
		`UPDATE batch_subjects SET subject_id = $1, teacher_id = $2, days = $3, start_time = $4, end_time = $5
		 WHERE id = $6 AND batch_id = $7`,
		req.SubjectID, req.TeacherID, req.Days, req.StartTime, req.EndTime, entryID, batchID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update subject"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "assignment not found"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "subject updated"})
}

func AdminUnassignBatchSubject(c *gin.Context) {
	batchID := c.Param("id")
	entryID := c.Param("entryId")

	tag, err := database.DB.Exec(context.Background(),
		`DELETE FROM batch_subjects WHERE id = $1 AND batch_id = $2`, entryID, batchID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "assignment not found"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "subject unassigned"})
}

func AdminGetBatchStats(c *gin.Context) {
	courseID := c.Query("course_id")
	if courseID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "course_id is required"})
		return
	}

	ctx := context.Background()
	rows, err := database.DB.Query(ctx,
		`SELECT b.id, b.name, COUNT(e.id) as student_count
		 FROM batches b
		 LEFT JOIN enrollments e ON e.batch_id = b.id AND e.status = 'approved'
		 WHERE b.course_id = $1
		 GROUP BY b.id, b.name ORDER BY b.name`, courseID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	type BatchStat struct {
		ID           int    `json:"id"`
		Name         string `json:"name"`
		StudentCount int    `json:"student_count"`
	}
	var stats []BatchStat
	for rows.Next() {
		var s BatchStat
		if err := rows.Scan(&s.ID, &s.Name, &s.StudentCount); err == nil {
			stats = append(stats, s)
		}
	}
	if stats == nil {
		stats = []BatchStat{}
	}
	c.JSON(http.StatusOK, stats)
}
