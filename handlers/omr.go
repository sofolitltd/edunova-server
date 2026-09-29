package handlers

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"edunova-server/database"
	"edunova-server/models"
	"edunova-server/services/omr"
)

func omrExamCodeExists(ctx context.Context, code string) (bool, error) {
	var exists bool
	err := database.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM omr_exams WHERE exam_code = $1)`, code).Scan(&exists)
	return exists, err
}

// generateOMRExamCode picks a random exam_code.ExamCodeDigitCount-digit
// numeric code and retries on collision. The code is printed (as bubbles)
// on every copy of the exam's sheet so Evaluate OMR can identify the exam
// from the photo alone.
func generateOMRExamCode(ctx context.Context) (string, error) {
	digits := omr.ExamCodeDigitCount
	max := 1
	for i := 0; i < digits; i++ {
		max *= 10
	}
	for attempt := 0; attempt < 20; attempt++ {
		code := fmt.Sprintf("%0*d", digits, rand.Intn(max))
		exists, err := omrExamCodeExists(ctx, code)
		if err != nil {
			return "", err
		}
		if !exists {
			return code, nil
		}
	}
	return "", fmt.Errorf("could not generate a unique exam code")
}

// AdminCreateOMRExam creates an OMR exam with its answer key.
func AdminCreateOMRExam(c *gin.Context) {
	var req models.CreateOMRExamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "title is required"})
		return
	}
	questionCount := len(req.Questions)
	columns := req.Columns
	if columns <= 0 {
		columns = omr.SuggestColumns(questionCount)
	}
	if ok, reason := omr.IsScannable(questionCount, columns); !ok {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: reason})
		return
	}

	seen := make(map[int]bool, questionCount)
	for _, q := range req.Questions {
		if q.QuestionNumber < 1 || q.QuestionNumber > questionCount {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "question numbers must be sequential from 1 to the question count"})
			return
		}
		if seen[q.QuestionNumber] {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: fmt.Sprintf("duplicate question number %d", q.QuestionNumber)})
			return
		}
		seen[q.QuestionNumber] = true
	}

	ctx := c.Request.Context()
	code, err := generateOMRExamCode(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to generate exam code"})
		return
	}

	adminIDRaw, _ := c.Get("admin_id")
	adminID, _ := adminIDRaw.(int)

	tx, err := database.DB.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer tx.Rollback(ctx)

	var exam models.OMRExam
	err = tx.QueryRow(ctx,
		`INSERT INTO omr_exams (title, class_level, subject, question_count, columns, exam_code, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, title, class_level, subject, question_count, columns, exam_code, created_at, updated_at`,
		req.Title, req.ClassLevel, req.Subject, questionCount, columns, code, adminID,
	).Scan(&exam.ID, &exam.Title, &exam.ClassLevel, &exam.Subject, &exam.QuestionCount, &exam.Columns, &exam.ExamCode, &exam.CreatedAt, &exam.UpdatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create exam"})
		return
	}

	for _, q := range req.Questions {
		if _, err := tx.Exec(ctx,
			`INSERT INTO omr_questions (omr_exam_id, question_number, correct_option) VALUES ($1, $2, $3)`,
			exam.ID, q.QuestionNumber, q.CorrectOption,
		); err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to save answer key"})
			return
		}
	}

	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create exam"})
		return
	}

	c.JSON(http.StatusCreated, exam)
}

// AdminListOMRExams lists all OMR exams with roster/sheet counts.
func AdminListOMRExams(c *gin.Context) {
	rows, err := database.DB.Query(c.Request.Context(), `
		SELECT e.id, e.title, e.class_level, e.subject, e.question_count, e.columns, e.exam_code,
		       COALESCE((SELECT COUNT(*) FROM omr_students s WHERE s.omr_exam_id = e.id), 0),
		       COALESCE((SELECT COUNT(*) FROM omr_sheets sh WHERE sh.omr_exam_id = e.id), 0),
		       e.created_at, e.updated_at
		FROM omr_exams e
		ORDER BY e.created_at DESC`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	exams := []models.OMRExam{}
	for rows.Next() {
		var e models.OMRExam
		if err := rows.Scan(&e.ID, &e.Title, &e.ClassLevel, &e.Subject, &e.QuestionCount, &e.Columns, &e.ExamCode,
			&e.StudentCount, &e.SheetCount, &e.CreatedAt, &e.UpdatedAt); err != nil {
			continue
		}
		exams = append(exams, e)
	}
	c.JSON(http.StatusOK, exams)
}

// AdminGetOMRExam returns one exam plus its answer key and roster.
func AdminGetOMRExam(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid exam id"})
		return
	}
	ctx := c.Request.Context()

	var e models.OMRExam
	err = database.DB.QueryRow(ctx, `
		SELECT e.id, e.title, e.class_level, e.subject, e.question_count, e.columns, e.exam_code,
		       COALESCE((SELECT COUNT(*) FROM omr_students s WHERE s.omr_exam_id = e.id), 0),
		       COALESCE((SELECT COUNT(*) FROM omr_sheets sh WHERE sh.omr_exam_id = e.id), 0),
		       e.created_at, e.updated_at
		FROM omr_exams e WHERE e.id = $1`, id,
	).Scan(&e.ID, &e.Title, &e.ClassLevel, &e.Subject, &e.QuestionCount, &e.Columns, &e.ExamCode,
		&e.StudentCount, &e.SheetCount, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "exam not found"})
		return
	}

	rows, err := database.DB.Query(ctx, `SELECT id, omr_exam_id, question_number, correct_option FROM omr_questions WHERE omr_exam_id = $1 ORDER BY question_number`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	questions := []models.OMRQuestion{}
	for rows.Next() {
		var q models.OMRQuestion
		if err := rows.Scan(&q.ID, &q.OMRExamID, &q.QuestionNumber, &q.CorrectOption); err == nil {
			questions = append(questions, q)
		}
	}
	rows.Close()

	sRows, err := database.DB.Query(ctx, `SELECT id, omr_exam_id, roll_number, name, enrollment_id FROM omr_students WHERE omr_exam_id = $1 ORDER BY roll_number`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	students := []models.OMRStudent{}
	for sRows.Next() {
		var s models.OMRStudent
		if err := sRows.Scan(&s.ID, &s.OMRExamID, &s.RollNumber, &s.Name, &s.EnrollmentID); err == nil {
			students = append(students, s)
		}
	}
	sRows.Close()

	c.JSON(http.StatusOK, gin.H{
		"exam":      e,
		"questions": questions,
		"students":  students,
	})
}

// AdminAddOMRStudents bulk-adds a roster (roll + name) to an exam.
func AdminAddOMRStudents(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid exam id"})
		return
	}
	var req models.AddOMRStudentsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	ctx := c.Request.Context()
	var examExists bool
	if err := database.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM omr_exams WHERE id = $1)`, id).Scan(&examExists); err != nil || !examExists {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "exam not found"})
		return
	}

	added := 0
	for _, s := range req.Students {
		roll := strings.TrimSpace(s.RollNumber)
		if roll == "" {
			continue
		}
		_, err := database.DB.Exec(ctx,
			`INSERT INTO omr_students (omr_exam_id, roll_number, name) VALUES ($1, $2, $3)
			 ON CONFLICT (omr_exam_id, roll_number) DO UPDATE SET name = EXCLUDED.name`,
			id, roll, strings.TrimSpace(s.Name),
		)
		if err == nil {
			added++
		}
	}
	c.JSON(http.StatusOK, gin.H{"added": added})
}

// AdminImportOMRRoster populates an OMR exam's roster from a batch's
// approved enrollments, linking each roster row back to its enrollment (and,
// where one exists, the enrolled student's app login) so a scored sheet can
// later be credited to a real student_results row.
func AdminImportOMRRoster(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid exam id"})
		return
	}
	var req models.ImportOMRRosterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	ctx := c.Request.Context()
	var examExists bool
	if err := database.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM omr_exams WHERE id = $1)`, id).Scan(&examExists); err != nil || !examExists {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "exam not found"})
		return
	}

	rows, err := database.DB.Query(ctx,
		`SELECT id, student_id, full_name, user_id FROM enrollments WHERE batch_id = $1 AND status = 'approved'`, req.BatchID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	imported := 0
	withoutLogin := 0
	for rows.Next() {
		var enrollmentID int
		var studentID, fullName string
		var userID *int
		if err := rows.Scan(&enrollmentID, &studentID, &fullName, &userID); err != nil {
			continue
		}
		roll := strings.TrimSpace(studentID)
		if roll == "" {
			continue
		}
		if _, err := database.DB.Exec(ctx,
			`INSERT INTO omr_students (omr_exam_id, roll_number, name, enrollment_id) VALUES ($1, $2, $3, $4)
			 ON CONFLICT (omr_exam_id, roll_number) DO UPDATE SET name = EXCLUDED.name, enrollment_id = EXCLUDED.enrollment_id`,
			id, roll, strings.TrimSpace(fullName), enrollmentID,
		); err != nil {
			continue
		}
		imported++
		if userID == nil {
			withoutLogin++
		}
	}

	c.JSON(http.StatusOK, gin.H{"imported": imported, "without_login": withoutLogin})
}

// AdminGetOMRTemplate returns the deterministic bubble-sheet layout for an
// exam, used by the admin web app to render/print the sheet and by this
// server to decode uploaded photos of it.
func AdminGetOMRTemplate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid exam id"})
		return
	}
	var questionCount, columns int
	var examCode string
	err = database.DB.QueryRow(c.Request.Context(),
		`SELECT question_count, columns, exam_code FROM omr_exams WHERE id = $1`, id,
	).Scan(&questionCount, &columns, &examCode)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "exam not found"})
		return
	}

	t := omr.BuildTemplate(questionCount, columns)
	c.JSON(http.StatusOK, gin.H{
		"template":  t,
		"exam_code": examCode,
	})
}

// AdminCheckOMRScannable is a live-validation endpoint mirroring the
// reference tool's "this OMR is scannable" check, so the admin UI can warn
// before the exam is created.
func AdminCheckOMRScannable(c *gin.Context) {
	questionCount, _ := strconv.Atoi(c.Query("question_count"))
	columns, _ := strconv.Atoi(c.Query("columns"))
	if columns <= 0 {
		columns = omr.SuggestColumns(questionCount)
	}
	ok, reason := omr.IsScannable(questionCount, columns)
	c.JSON(http.StatusOK, gin.H{"scannable": ok, "reason": reason, "columns": columns})
}

// AdminPreviewOMRTemplate returns the deterministic layout for a given
// question count/columns without requiring a saved exam, so the admin UI
// can render a live preview while the form is still being filled in.
func AdminPreviewOMRTemplate(c *gin.Context) {
	questionCount, _ := strconv.Atoi(c.Query("question_count"))
	columns, _ := strconv.Atoi(c.Query("columns"))
	if questionCount < omr.MinQuestions {
		questionCount = omr.MinQuestions
	}
	if questionCount > omr.MaxQuestions {
		questionCount = omr.MaxQuestions
	}
	t := omr.BuildTemplate(questionCount, columns)
	c.JSON(http.StatusOK, gin.H{"template": t})
}
