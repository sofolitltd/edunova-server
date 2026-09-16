package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"edunova-server/database"
	"edunova-server/models"
)

func AdminGetDoubts(c *gin.Context) {
	status := c.Query("status")
	studentID := c.Query("student_id")
	batchID := c.Query("batch_id")
	query := `
		SELECT d.id, d.student_id, u.full_name, d.question_text, COALESCE(d.subject,''),
		 COALESCE(d.chapter,''), COALESCE(d.image_url,''), d.status, COALESCE(d.resolution,''),
		 d.resolved_by, COALESCE(au.full_name,''), d.resolved_at,
		 d.parent_notified, d.student_notified, d.created_at
		FROM doubts d
		JOIN users u ON d.student_id = u.id
		LEFT JOIN admin_users au ON d.resolved_by = au.id`
	args := []interface{}{}
	conditions := []string{}

	if status != "" {
		args = append(args, status)
		conditions = append(conditions, `d.status = $`+strconv.Itoa(len(args)))
	}
	if studentID != "" {
		args = append(args, studentID)
		conditions = append(conditions, `d.student_id = $`+strconv.Itoa(len(args)))
	}
	if batchID != "" {
		args = append(args, batchID)
		conditions = append(conditions, `d.student_id IN (
			SELECT e.user_id FROM enrollments e WHERE e.batch_id = $`+strconv.Itoa(len(args))+` AND e.status = 'approved'
		)`)
	}
	if teacherIDs, isTeacher := teacherAssignedBatchIDs(c); isTeacher {
		if len(teacherIDs) == 0 || (batchID != "" && !intSliceContains(teacherIDs, batchID)) {
			c.JSON(http.StatusOK, []models.Doubt{})
			return
		}
		if batchID == "" {
			args = append(args, teacherIDs)
			conditions = append(conditions, `d.student_id IN (
				SELECT e.user_id FROM enrollments e WHERE e.batch_id = ANY($`+strconv.Itoa(len(args))+`) AND e.status = 'approved'
			)`)
		}
	}
	if len(conditions) > 0 {
		query += ` WHERE ` + strings.Join(conditions, " AND ")
	}
	query += ` ORDER BY d.created_at DESC`

	rows, err := database.DB.Query(context.Background(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch doubts"})
		return
	}
	defer rows.Close()

	var doubts []models.Doubt
	for rows.Next() {
		var d models.Doubt
		if err := rows.Scan(&d.ID, &d.StudentID, &d.StudentName, &d.QuestionText, &d.Subject,
			&d.Chapter, &d.ImageURL, &d.Status, &d.Resolution,
			&d.ResolvedBy, &d.ResolvedByName, &d.ResolvedAt,
			&d.ParentNotified, &d.StudentNotified, &d.CreatedAt); err != nil {
			continue
		}
		doubts = append(doubts, d)
	}
	if doubts == nil {
		doubts = []models.Doubt{}
	}
	c.JSON(http.StatusOK, doubts)
}

func StudentSubmitDoubt(c *gin.Context) {
	mobile := c.GetString("mobile")
	var req models.CreateDoubtRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var studentID int
	err := database.DB.QueryRow(context.Background(), `SELECT id FROM users WHERE mobile=$1`, mobile).Scan(&studentID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "student not found"})
		return
	}

	var d models.Doubt
	err = database.DB.QueryRow(context.Background(),
		`INSERT INTO doubts (student_id, question_text, subject, chapter, image_url)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, student_id, question_text, COALESCE(subject,''), COALESCE(chapter,''),
		 COALESCE(image_url,''), status, created_at`,
		studentID, req.QuestionText, req.Subject, req.Chapter, req.ImageURL,
	).Scan(&d.ID, &d.StudentID, &d.QuestionText, &d.Subject, &d.Chapter,
		&d.ImageURL, &d.Status, &d.CreatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to submit doubt"})
		return
	}
	d.StudentName = ""
	c.JSON(http.StatusCreated, d)
}

func AdminResolveDoubt(c *gin.Context) {
	id := c.Param("id")
	adminID, _ := c.Get("admin_id")

	if !teacherOwnsDoubt(c, id) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "not authorized for this doubt"})
		return
	}

	var req models.ResolveDoubtRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	_, err := database.DB.Exec(context.Background(),
		`UPDATE doubts SET status='resolved', resolution=$1, resolved_by=$2, resolved_at=NOW() WHERE id=$3`,
		req.Resolution, adminID, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to resolve doubt"})
		return
	}

	// Notify student via FCM
	var studentID int
	database.DB.QueryRow(context.Background(), `SELECT student_id FROM doubts WHERE id=$1`, id).Scan(&studentID)
	if studentID > 0 {
		go sendDoubtNotification(studentID, "Your doubt has been resolved! Check the answer now.")
	}

	c.JSON(http.StatusOK, models.SuccessResponse{Message: "doubt resolved"})
}

func AdminCloseDoubt(c *gin.Context) {
	id := c.Param("id")
	if !teacherOwnsDoubt(c, id) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "not authorized for this doubt"})
		return
	}
	_, err := database.DB.Exec(context.Background(),
		`UPDATE doubts SET status='closed' WHERE id=$1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to close doubt"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "doubt closed"})
}

func sendDoubtNotification(studentID int, message string) {
	var tokens []string
	rows, _ := database.DB.Query(context.Background(), `SELECT token FROM device_tokens WHERE user_id=$1`, studentID)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var t string
			if rows.Scan(&t) == nil {
				tokens = append(tokens, t)
			}
		}
	}
	for _, token := range tokens {
		_ = sendPushNotification(token, "EduNova Doubt", message)
	}
}

func sendPushNotification(token, title, body string) error {
	// Placeholder — real FCM via services.SendFCMV1
	return fmt.Errorf("FCM not configured")
}

func AdminGetDoubtStats(c *gin.Context) {
	var total, pending, resolved, closed int
	ctx := context.Background()

	if teacherIDs, isTeacher := teacherAssignedBatchIDs(c); isTeacher {
		if len(teacherIDs) == 0 {
			c.JSON(http.StatusOK, gin.H{"total": 0, "pending": 0, "resolved": 0, "closed": 0})
			return
		}
		batchFilter := `WHERE student_id IN (
			SELECT e.user_id FROM enrollments e WHERE e.batch_id = ANY($1) AND e.status = 'approved'
		)`
		database.DB.QueryRow(ctx, `SELECT COUNT(*) FROM doubts `+batchFilter, teacherIDs).Scan(&total)
		database.DB.QueryRow(ctx, `SELECT COUNT(*) FROM doubts `+batchFilter+` AND status='pending'`, teacherIDs).Scan(&pending)
		database.DB.QueryRow(ctx, `SELECT COUNT(*) FROM doubts `+batchFilter+` AND status='resolved'`, teacherIDs).Scan(&resolved)
		database.DB.QueryRow(ctx, `SELECT COUNT(*) FROM doubts `+batchFilter+` AND status='closed'`, teacherIDs).Scan(&closed)
		c.JSON(http.StatusOK, gin.H{"total": total, "pending": pending, "resolved": resolved, "closed": closed})
		return
	}

	database.DB.QueryRow(ctx, `SELECT COUNT(*) FROM doubts`).Scan(&total)
	database.DB.QueryRow(ctx, `SELECT COUNT(*) FROM doubts WHERE status='pending'`).Scan(&pending)
	database.DB.QueryRow(ctx, `SELECT COUNT(*) FROM doubts WHERE status='resolved'`).Scan(&resolved)
	database.DB.QueryRow(ctx, `SELECT COUNT(*) FROM doubts WHERE status='closed'`).Scan(&closed)
	c.JSON(http.StatusOK, gin.H{"total": total, "pending": pending, "resolved": resolved, "closed": closed})
}

// Alias for route compatibility
func StudentGetMyDoubts(c *gin.Context) {
	mobile := c.GetString("mobile")
	rows, err := database.DB.Query(context.Background(),
		`SELECT d.id, d.student_id, u.full_name, d.question_text, COALESCE(d.subject,''),
		 COALESCE(d.chapter,''), COALESCE(d.image_url,''), d.status, COALESCE(d.resolution,''),
		 d.resolved_by, COALESCE(au.full_name,''), d.resolved_at,
		 d.parent_notified, d.student_notified, d.created_at
		 FROM doubts d JOIN users u ON d.student_id = u.id
		 LEFT JOIN admin_users au ON d.resolved_by = au.id
		 WHERE u.mobile = $1 ORDER BY d.created_at DESC`, mobile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch doubts"})
		return
	}
	defer rows.Close()

	var doubts []models.Doubt
	for rows.Next() {
		var d models.Doubt
		if err := rows.Scan(&d.ID, &d.StudentID, &d.StudentName, &d.QuestionText, &d.Subject,
			&d.Chapter, &d.ImageURL, &d.Status, &d.Resolution,
			&d.ResolvedBy, &d.ResolvedByName, &d.ResolvedAt,
			&d.ParentNotified, &d.StudentNotified, &d.CreatedAt); err != nil {
			continue
		}
		doubts = append(doubts, d)
	}
	if doubts == nil {
		doubts = []models.Doubt{}
	}
	c.JSON(http.StatusOK, doubts)
}

func AdminGetTodayLessons(c *gin.Context) {
	today := time.Now().Format("2006-01-02")
	batchID := c.Query("batch_id")

	query := `SELECT l.id, l.course_id, COALESCE(c.title,''), l.batch_id, COALESCE(b.name,''),
		 l.title, COALESCE(l.description,''), COALESCE(l.subject,''), COALESCE(l.chapter,''),
		 l.lesson_date, COALESCE(l.teacher_notes,''), l.created_by, l.created_at
		 FROM lessons l
		 LEFT JOIN courses c ON l.course_id = c.id
		 LEFT JOIN batches b ON l.batch_id = b.id`
	args := []interface{}{today}
	conditions := []string{"l.lesson_date = $1"}

	if batchID != "" {
		args = append(args, batchID)
		conditions = append(conditions, fmt.Sprintf("l.batch_id = $%d", len(args)))
	}
	if teacherIDs, isTeacher := teacherAssignedBatchIDs(c); isTeacher {
		if len(teacherIDs) == 0 || (batchID != "" && !intSliceContains(teacherIDs, batchID)) {
			c.JSON(http.StatusOK, []models.Lesson{})
			return
		}
		if batchID == "" {
			args = append(args, teacherIDs)
			conditions = append(conditions, fmt.Sprintf("l.batch_id = ANY($%d)", len(args)))
		}
	}
	query += " WHERE " + strings.Join(conditions, " AND ") + ` ORDER BY l.created_at DESC`

	rows, err := database.DB.Query(context.Background(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch lessons"})
		return
	}
	defer rows.Close()

	var lessons []models.Lesson
	for rows.Next() {
		var l models.Lesson
		if err := rows.Scan(&l.ID, &l.CourseID, &l.CourseName, &l.BatchID, &l.BatchName,
			&l.Title, &l.Description, &l.Subject, &l.Chapter,
			&l.LessonDate, &l.TeacherNotes, &l.CreatedBy, &l.CreatedAt); err != nil {
			continue
		}
		lessons = append(lessons, l)
	}
	if lessons == nil {
		lessons = []models.Lesson{}
	}
	c.JSON(http.StatusOK, lessons)
}

// User-facing today lessons (for student/parent dashboard)
func UserGetTodayLessons(c *gin.Context) {
	today := time.Now().Format("2006-01-02")
	rows, err := database.DB.Query(context.Background(),
		`SELECT l.id, l.course_id, COALESCE(c.title,''), l.title, COALESCE(l.description,''),
		 COALESCE(l.subject,''), COALESCE(l.chapter,''), l.lesson_date
		 FROM lessons l
		 LEFT JOIN courses c ON l.course_id = c.id
		 WHERE l.lesson_date = $1
		 ORDER BY l.created_at DESC`, today)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch lessons"})
		return
	}
	defer rows.Close()

	type LessonBrief struct {
		ID          int    `json:"id"`
		CourseID    int    `json:"course_id"`
		CourseName  string `json:"course_name"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Subject     string `json:"subject"`
		Chapter     string `json:"chapter"`
		LessonDate  string `json:"lesson_date"`
	}

	var lessons []LessonBrief
	for rows.Next() {
		var l LessonBrief
		if err := rows.Scan(&l.ID, &l.CourseID, &l.CourseName, &l.Title, &l.Description,
			&l.Subject, &l.Chapter, &l.LessonDate); err != nil {
			continue
		}
		lessons = append(lessons, l)
	}
	if lessons == nil {
		lessons = []LessonBrief{}
	}
	c.JSON(http.StatusOK, lessons)
}

func AdminGetCalendarEvents(c *gin.Context) {
	month := c.Query("month")
	query := `
		SELECT ce.id, ce.title, COALESCE(ce.description,''), ce.event_type, ce.date,
		 COALESCE(ce.end_date,''), ce.course_id, COALESCE(c.title,''),
		 ce.batch_id, COALESCE(b.name,''), COALESCE(ce.color,'#6366F1'),
		 ce.is_auto, ce.notification_sent, ce.created_by, ce.created_at
		 FROM calendar_events ce
		 LEFT JOIN courses c ON ce.course_id = c.id
		 LEFT JOIN batches b ON ce.batch_id = b.id`
	args := []interface{}{}

	if month != "" {
		query += ` WHERE TO_CHAR(ce.date, 'YYYY-MM') = $1`
		args = append(args, month)
	}
	query += ` ORDER BY ce.date ASC`

	rows, err := database.DB.Query(context.Background(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch events"})
		return
	}
	defer rows.Close()

	var events []models.CalendarEvent
	for rows.Next() {
		var e models.CalendarEvent
		if err := rows.Scan(&e.ID, &e.Title, &e.Description, &e.EventType, &e.Date,
			&e.EndDate, &e.CourseID, &e.CourseName,
			&e.BatchID, &e.BatchName, &e.Color,
			&e.IsAuto, &e.NotificationSent, &e.CreatedBy, &e.CreatedAt); err != nil {
			continue
		}
		events = append(events, e)
	}
	if events == nil {
		events = []models.CalendarEvent{}
	}
	c.JSON(http.StatusOK, events)
}

func AdminCreateCalendarEvent(c *gin.Context) {
	var req models.CreateCalendarEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	adminID, _ := c.Get("admin_id")
	color := req.Color
	if color == "" {
		color = "#6366F1"
	}

	var e models.CalendarEvent
	err := database.DB.QueryRow(context.Background(),
		`INSERT INTO calendar_events (title, description, event_type, date, end_date, course_id, batch_id, color, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, title, description, event_type, date, COALESCE(end_date,''), course_id, batch_id, color, is_auto, created_at`,
		req.Title, req.Description, req.EventType, req.Date, req.EndDate,
		req.CourseID, req.BatchID, color, adminID,
	).Scan(&e.ID, &e.Title, &e.Description, &e.EventType, &e.Date,
		&e.EndDate, &e.CourseID, &e.BatchID, &e.Color, &e.IsAuto, &e.CreatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create event"})
		return
	}
	c.JSON(http.StatusCreated, e)
}

func AdminUpdateCalendarEvent(c *gin.Context) {
	id := c.Param("id")
	var req models.CreateCalendarEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	color := req.Color
	if color == "" {
		color = "#6366F1"
	}

	_, err := database.DB.Exec(context.Background(),
		`UPDATE calendar_events SET title=$1, description=$2, event_type=$3, date=$4, end_date=$5,
		 course_id=$6, batch_id=$7, color=$8 WHERE id=$9`,
		req.Title, req.Description, req.EventType, req.Date, req.EndDate,
		req.CourseID, req.BatchID, color, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update event"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "event updated"})
}

func AdminDeleteCalendarEvent(c *gin.Context) {
	id := c.Param("id")
	_, err := database.DB.Exec(context.Background(), `DELETE FROM calendar_events WHERE id=$1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to delete event"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "event deleted"})
}

func AdminGetLessons(c *gin.Context) {
	batchID := c.Query("batch_id")

	query := `SELECT l.id, l.course_id, COALESCE(c.title,''), l.batch_id, COALESCE(b.name,''),
		 l.title, COALESCE(l.description,''), COALESCE(l.subject,''), COALESCE(l.chapter,''),
		 l.lesson_date, COALESCE(l.teacher_notes,''), l.created_by, l.created_at
		 FROM lessons l
		 LEFT JOIN courses c ON l.course_id = c.id
		 LEFT JOIN batches b ON l.batch_id = b.id`
	var args []interface{}
	var conditions []string

	if batchID != "" {
		args = append(args, batchID)
		conditions = append(conditions, fmt.Sprintf("l.batch_id = $%d", len(args)))
	}
	if teacherIDs, isTeacher := teacherAssignedBatchIDs(c); isTeacher {
		if len(teacherIDs) == 0 || (batchID != "" && !intSliceContains(teacherIDs, batchID)) {
			c.JSON(http.StatusOK, []models.Lesson{})
			return
		}
		if batchID == "" {
			args = append(args, teacherIDs)
			conditions = append(conditions, fmt.Sprintf("l.batch_id = ANY($%d)", len(args)))
		}
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += ` ORDER BY l.lesson_date DESC, l.created_at DESC`

	rows, err := database.DB.Query(context.Background(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch lessons"})
		return
	}
	defer rows.Close()

	var lessons []models.Lesson
	for rows.Next() {
		var l models.Lesson
		if err := rows.Scan(&l.ID, &l.CourseID, &l.CourseName, &l.BatchID, &l.BatchName,
			&l.Title, &l.Description, &l.Subject, &l.Chapter,
			&l.LessonDate, &l.TeacherNotes, &l.CreatedBy, &l.CreatedAt); err != nil {
			continue
		}
		lessons = append(lessons, l)
	}
	if lessons == nil {
		lessons = []models.Lesson{}
	}
	c.JSON(http.StatusOK, lessons)
}

func AdminCreateLesson(c *gin.Context) {
	var req models.CreateLessonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	if !teacherCanUseBatchID(c, req.BatchID) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "not authorized for this batch"})
		return
	}
	adminID, _ := c.Get("admin_id")

	var l models.Lesson
	err := database.DB.QueryRow(context.Background(),
		`INSERT INTO lessons (course_id, batch_id, title, description, subject, chapter, lesson_date, teacher_notes, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, course_id, title, description, subject, chapter, lesson_date, teacher_notes, created_at`,
		req.CourseID, req.BatchID, req.Title, req.Description, req.Subject,
		req.Chapter, req.LessonDate, req.TeacherNotes, adminID,
	).Scan(&l.ID, &l.CourseID, &l.Title, &l.Description, &l.Subject,
		&l.Chapter, &l.LessonDate, &l.TeacherNotes, &l.CreatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create lesson"})
		return
	}
	c.JSON(http.StatusCreated, l)
}

func AdminUpdateLesson(c *gin.Context) {
	id := c.Param("id")
	var req models.CreateLessonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	if !teacherOwnsBatchRow(c, "lessons", id) || !teacherCanUseBatchID(c, req.BatchID) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "not authorized for this batch"})
		return
	}

	_, err := database.DB.Exec(context.Background(),
		`UPDATE lessons SET course_id=$1, batch_id=$2, title=$3, description=$4, subject=$5,
		 chapter=$6, lesson_date=$7, teacher_notes=$8 WHERE id=$9`,
		req.CourseID, req.BatchID, req.Title, req.Description, req.Subject,
		req.Chapter, req.LessonDate, req.TeacherNotes, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update lesson"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "lesson updated"})
}

func AdminDeleteLesson(c *gin.Context) {
	id := c.Param("id")
	if !teacherOwnsBatchRow(c, "lessons", id) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "not authorized for this batch"})
		return
	}
	_, err := database.DB.Exec(context.Background(), `DELETE FROM lessons WHERE id=$1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to delete lesson"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "lesson deleted"})
}

func AdminGetPayments(c *gin.Context) {
	status := c.Query("status")
	userID := c.Query("user_id")
	query := `
		SELECT p.id, p.user_id, u.full_name, u.mobile, p.enrollment_id, p.course_id,
		 COALESCE(c.title,''), p.amount, p.method, COALESCE(p.transaction_id,''),
		 COALESCE(p.sender_number,''), COALESCE(p.receiver_number,''), p.status,
		 COALESCE(p.receipt_number,''), COALESCE(p.month,''), p.year,
		 COALESCE(p.notes,''), p.verified_by, COALESCE(au.full_name,''),
		 p.verified_at, p.created_at
		 FROM payments p
		 JOIN users u ON p.user_id = u.id
		 LEFT JOIN courses c ON p.course_id = c.id
		 LEFT JOIN admin_users au ON p.verified_by = au.id`
	args := []interface{}{}
	conditions := []string{}

	if status != "" {
		args = append(args, status)
		conditions = append(conditions, `p.status = $`+strconv.Itoa(len(args)))
	}
	if userID != "" {
		args = append(args, userID)
		conditions = append(conditions, `p.user_id = $`+strconv.Itoa(len(args)))
	}
	if len(conditions) > 0 {
		query += ` WHERE ` + strings.Join(conditions, " AND ")
	}
	query += ` ORDER BY p.created_at DESC`

	rows, err := database.DB.Query(context.Background(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch payments"})
		return
	}
	defer rows.Close()

	var payments []models.Payment
	for rows.Next() {
		var p models.Payment
		var enrollmentID, courseID, verifiedBy *int
		if err := rows.Scan(&p.ID, &p.UserID, &p.UserName, &p.UserMobile, &enrollmentID,
			&courseID, &p.CourseName, &p.Amount, &p.Method, &p.TransactionID,
			&p.SenderNumber, &p.ReceiverNumber, &p.Status, &p.ReceiptNumber,
			&p.Month, &p.Year, &p.Notes, &verifiedBy, &p.VerifiedByName,
			&p.VerifiedAt, &p.CreatedAt); err != nil {
			continue
		}
		if enrollmentID != nil {
			p.EnrollmentID = *enrollmentID
		}
		if courseID != nil {
			p.CourseID = *courseID
		}
		if verifiedBy != nil {
			p.VerifiedBy = *verifiedBy
		}
		payments = append(payments, p)
	}
	if payments == nil {
		payments = []models.Payment{}
	}
	c.JSON(http.StatusOK, payments)
}

func AdminCreatePayment(c *gin.Context) {
	var req models.CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	receiptNo := fmt.Sprintf("EDU-%d-%04d", time.Now().Unix()%100000, time.Now().UnixNano()%10000)

	var p models.Payment
	err := database.DB.QueryRow(context.Background(),
		`INSERT INTO payments (user_id, enrollment_id, course_id, amount, method, transaction_id,
		 sender_number, receiver_number, receipt_number, month, year, notes)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		 RETURNING id, user_id, amount, method, transaction_id, receipt_number, month, year, status, created_at`,
		req.UserID, req.EnrollmentID, req.CourseID, req.Amount, req.Method,
		req.TransactionID, req.SenderNumber, req.ReceiverNumber,
		receiptNo, req.Month, req.Year, req.Notes,
	).Scan(&p.ID, &p.UserID, &p.Amount, &p.Method, &p.TransactionID,
		&p.ReceiptNumber, &p.Month, &p.Year, &p.Status, &p.CreatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create payment"})
		return
	}
	c.JSON(http.StatusCreated, p)
}

func AdminVerifyPayment(c *gin.Context) {
	id := c.Param("id")
	adminID, _ := c.Get("admin_id")

	_, err := database.DB.Exec(context.Background(),
		`UPDATE payments SET status='verified', verified_by=$1, verified_at=NOW() WHERE id=$2`,
		adminID, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to verify payment"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "payment verified"})
}

func AdminRejectPayment(c *gin.Context) {
	id := c.Param("id")
	_, err := database.DB.Exec(context.Background(),
		`UPDATE payments SET status='rejected' WHERE id=$1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to reject payment"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "payment rejected"})
}

func AdminDeletePayment(c *gin.Context) {
	id := c.Param("id")
	_, err := database.DB.Exec(context.Background(), `DELETE FROM payments WHERE id=$1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to delete payment"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "payment deleted"})
}

func AdminGetArticles(c *gin.Context) {
	rows, err := database.DB.Query(context.Background(),
		`SELECT id, title, content, COALESCE(category,'general'),
		 COALESCE(video_url,''), COALESCE(image_url,''), is_published, COALESCE(created_by,0), created_at
		 FROM articles ORDER BY created_at DESC`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch articles"})
		return
	}
	defer rows.Close()

	var articles []models.Article
	for rows.Next() {
		var a models.Article
		if err := rows.Scan(&a.ID, &a.Title, &a.Content, &a.Category,
			&a.VideoURL, &a.ImageURL, &a.Published, &a.CreatedBy, &a.CreatedAt); err != nil {
			continue
		}
		articles = append(articles, a)
	}
	if articles == nil {
		articles = []models.Article{}
	}
	c.JSON(http.StatusOK, articles)
}

func AdminCreateArticle(c *gin.Context) {
	var req models.CreateArticleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	adminID, _ := c.Get("admin_id")
	category := req.Category
	if category == "" {
		category = "general"
	}

	var a models.Article
	err := database.DB.QueryRow(context.Background(),
		`INSERT INTO articles (title, content, category, video_url, image_url, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, title, content, category, video_url, image_url, is_published, created_at`,
		req.Title, req.Content, category, req.VideoURL, req.ImageURL, adminID,
	).Scan(&a.ID, &a.Title, &a.Content, &a.Category,
		&a.VideoURL, &a.ImageURL, &a.Published, &a.CreatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create article"})
		return
	}
	c.JSON(http.StatusCreated, a)
}

func AdminUpdateArticle(c *gin.Context) {
	id := c.Param("id")
	var req models.CreateArticleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	_, err := database.DB.Exec(context.Background(),
		`UPDATE articles SET title=$1, content=$2, category=$3, video_url=$4, image_url=$5 WHERE id=$6`,
		req.Title, req.Content, req.Category, req.VideoURL, req.ImageURL, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update article"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "article updated"})
}

func AdminDeleteArticle(c *gin.Context) {
	id := c.Param("id")
	_, err := database.DB.Exec(context.Background(), `DELETE FROM articles WHERE id=$1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to delete article"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "article deleted"})
}

func AdminToggleArticlePublish(c *gin.Context) {
	id := c.Param("id")
	_, err := database.DB.Exec(context.Background(),
		`UPDATE articles SET is_published = NOT is_published WHERE id=$1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to toggle article"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "article toggled"})
}

// User-facing endpoints
func UserGetArticles(c *gin.Context) {
	rows, err := database.DB.Query(context.Background(),
		`SELECT id, title, content, COALESCE(category,'general'),
		 COALESCE(video_url,''), COALESCE(image_url,''), created_at
		 FROM articles WHERE is_published=true ORDER BY created_at DESC`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch articles"})
		return
	}
	defer rows.Close()

	type ArticleBrief struct {
		ID        int       `json:"id"`
		Title     string    `json:"title"`
		Content   string    `json:"content"`
		Category  string    `json:"category"`
		VideoURL  string    `json:"video_url"`
		ImageURL  string    `json:"image_url"`
		CreatedAt time.Time `json:"created_at"`
	}

	var articles []ArticleBrief
	for rows.Next() {
		var a ArticleBrief
		if err := rows.Scan(&a.ID, &a.Title, &a.Content, &a.Category,
			&a.VideoURL, &a.ImageURL, &a.CreatedAt); err != nil {
			continue
		}
		articles = append(articles, a)
	}
	if articles == nil {
		articles = []ArticleBrief{}
	}
	c.JSON(http.StatusOK, articles)
}

func UserGetUpcomingEvents(c *gin.Context) {
	today := time.Now().Format("2006-01-02")
	rows, err := database.DB.Query(context.Background(),
		`SELECT ce.id, ce.title, COALESCE(ce.description,''), ce.event_type, ce.date,
		 COALESCE(ce.color,'#6366F1')
		 FROM calendar_events ce
		 WHERE ce.date >= $1
		 ORDER BY ce.date ASC LIMIT 20`, today)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch events"})
		return
	}
	defer rows.Close()

	type EventBrief struct {
		ID          int    `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		EventType   string `json:"event_type"`
		Date        string `json:"date"`
		Color       string `json:"color"`
	}

	var events []EventBrief
	for rows.Next() {
		var e EventBrief
		if err := rows.Scan(&e.ID, &e.Title, &e.Description, &e.EventType, &e.Date, &e.Color); err != nil {
			continue
		}
		events = append(events, e)
	}
	if events == nil {
		events = []EventBrief{}
	}
	c.JSON(http.StatusOK, events)
}
