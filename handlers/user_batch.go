package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"edunova-server/database"
	"edunova-server/models"
)

// userEnrolledInBatch reports whether the student (by mobile) has an
// approved enrollment in the given batch — used to gate the batch detail
// endpoints below to students who actually belong to that batch.
func userEnrolledInBatch(ctx context.Context, mobile string, batchID int) bool {
	var exists bool
	_ = database.DB.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM enrollments WHERE mobile=$1 AND batch_id=$2 AND status='approved')`,
		mobile, batchID).Scan(&exists)
	return exists
}

// UserGetBatchDetail returns real batch info (schedule, course, teachers,
// student count) for a batch the logged-in student is enrolled in.
func UserGetBatchDetail(c *gin.Context) {
	mobile := c.GetString("mobile")
	batchID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid batch id"})
		return
	}

	ctx := context.Background()
	if !userEnrolledInBatch(ctx, mobile, batchID) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "not enrolled in this batch"})
		return
	}

	var (
		name, code, schedule, classLevel, shift, batchType, courseTitle, courseDesc string
		days                                                                        []string
		startTime, endTime                                                          string
		courseID                                                                    int
	)
	err = database.DB.QueryRow(ctx,
		`SELECT b.name, COALESCE(b.code, ''), COALESCE(b.days, '{}'), COALESCE(b.start_time, ''), COALESCE(b.end_time, ''),
		        COALESCE(b.schedule, ''), COALESCE(b.class_level, ''), COALESCE(b.shift, ''), COALESCE(b.type, ''),
		        COALESCE(b.course_id, 0), COALESCE(c.title, ''), COALESCE(c.description, '')
		 FROM batches b LEFT JOIN courses c ON b.course_id = c.id
		 WHERE b.id = $1`, batchID,
	).Scan(&name, &code, &days, &startTime, &endTime, &schedule, &classLevel, &shift, &batchType, &courseID, &courseTitle, &courseDesc)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "batch not found"})
		return
	}

	type teacherInfo struct {
		ID          int    `json:"id"`
		FullName    string `json:"full_name"`
		DisplayName string `json:"display_name"`
	}
	var teachers []teacherInfo
	rows, err := database.DB.Query(ctx,
		`SELECT t.id, t.full_name, COALESCE(t.nickname, ''), COALESCE(t.gender, '') FROM batch_teachers bt JOIN teachers t ON bt.teacher_id = t.id WHERE bt.batch_id = $1`,
		batchID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var t teacherInfo
			var nickname, gender string
			if err := rows.Scan(&t.ID, &t.FullName, &nickname, &gender); err == nil {
				t.DisplayName = models.TeacherDisplayName(t.FullName, nickname, gender)
				teachers = append(teachers, t)
			}
		}
	}
	if teachers == nil {
		teachers = []teacherInfo{}
	}

	var studentCount int
	_ = database.DB.QueryRow(ctx,
		`SELECT COUNT(*) FROM enrollments WHERE batch_id=$1 AND status='approved'`, batchID).Scan(&studentCount)

	c.JSON(http.StatusOK, gin.H{
		"id":                 batchID,
		"name":               name,
		"code":               code,
		"days":               days,
		"start_time":         startTime,
		"end_time":           endTime,
		"schedule":           schedule,
		"class_level":        classLevel,
		"shift":              shift,
		"type":               batchType,
		"course_id":          courseID,
		"course_name":        courseTitle,
		"course_description": courseDesc,
		"teachers":           teachers,
		"student_count":      studentCount,
	})
}

// UserGetBatchExams returns exams for a batch the student is enrolled in,
// including exams targeted at the whole course when no batch-specific exam exists.
func UserGetBatchExams(c *gin.Context) {
	mobile := c.GetString("mobile")
	batchID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid batch id"})
		return
	}

	ctx := context.Background()
	if !userEnrolledInBatch(ctx, mobile, batchID) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "not enrolled in this batch"})
		return
	}

	var courseID int
	_ = database.DB.QueryRow(ctx, `SELECT COALESCE(course_id, 0) FROM batches WHERE id=$1`, batchID).Scan(&courseID)

	rows, err := database.DB.Query(ctx,
		`SELECT id, title, date, time, duration, total_questions, COALESCE(is_live, false)
		 FROM exams
		 WHERE batch_id = $1 OR (batch_id IS NULL AND course_id = $2)
		 ORDER BY is_live DESC, date DESC`, batchID, courseID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	type examInfo struct {
		ID             int    `json:"id"`
		Title          string `json:"title"`
		Date           string `json:"date"`
		Time           string `json:"time"`
		Duration       string `json:"duration"`
		TotalQuestions int    `json:"total_questions"`
		IsLive         bool   `json:"is_live"`
	}
	var exams []examInfo
	for rows.Next() {
		var e examInfo
		if err := rows.Scan(&e.ID, &e.Title, &e.Date, &e.Time, &e.Duration, &e.TotalQuestions, &e.IsLive); err == nil {
			exams = append(exams, e)
		}
	}
	if exams == nil {
		exams = []examInfo{}
	}
	c.JSON(http.StatusOK, exams)
}

// UserGetBatchSubjects returns the per-subject weekly schedule (days,
// time, teacher) for a batch the student is enrolled in.
func UserGetBatchSubjects(c *gin.Context) {
	mobile := c.GetString("mobile")
	batchID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid batch id"})
		return
	}

	ctx := context.Background()
	if !userEnrolledInBatch(ctx, mobile, batchID) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "not enrolled in this batch"})
		return
	}

	rows, err := database.DB.Query(ctx,
		`SELECT bs.id, bs.subject_id, s.name, bs.teacher_id, COALESCE(t.full_name, ''), COALESCE(t.nickname, ''), COALESCE(t.gender, ''), COALESCE(bs.days, '{}'), COALESCE(bs.start_time, ''), COALESCE(bs.end_time, '')
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
		var nickname, gender string
		if err := rows.Scan(&s.ID, &s.SubjectID, &s.SubjectName, &s.TeacherID, &s.TeacherName, &nickname, &gender, &s.Days, &s.StartTime, &s.EndTime); err != nil {
			continue
		}
		s.TeacherDisplayName = models.TeacherDisplayName(s.TeacherName, nickname, gender)
		s.TeacherDisplayNameEn = models.TeacherDisplayNameEn(s.TeacherName, nickname, gender)
		subjects = append(subjects, s)
	}
	if subjects == nil {
		subjects = []models.BatchSubject{}
	}
	c.JSON(http.StatusOK, subjects)
}

// UserGetBatchLeaderboard ranks a batch's enrolled students by total exam score.
func UserGetBatchLeaderboard(c *gin.Context) {
	mobile := c.GetString("mobile")
	batchID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid batch id"})
		return
	}

	ctx := context.Background()
	if !userEnrolledInBatch(ctx, mobile, batchID) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "not enrolled in this batch"})
		return
	}

	var courseID int
	_ = database.DB.QueryRow(ctx, `SELECT COALESCE(course_id, 0) FROM batches WHERE id=$1`, batchID).Scan(&courseID)

	rows, err := database.DB.Query(ctx,
		`SELECT u.id, u.full_name, COALESCE(SUM(er.score), 0) AS total_score, COUNT(er.id) AS exams_taken
		 FROM enrollments e
		 JOIN users u ON u.id = e.user_id
		 LEFT JOIN exam_results er ON er.user_id = u.id
		   AND er.exam_id IN (SELECT id FROM exams WHERE batch_id = $1 OR (batch_id IS NULL AND course_id = $2))
		 WHERE e.batch_id = $1 AND e.status = 'approved' AND e.user_id IS NOT NULL
		 GROUP BY u.id, u.full_name
		 ORDER BY total_score DESC, exams_taken DESC
		 LIMIT 50`, batchID, courseID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	type leaderboardEntry struct {
		UserID     int    `json:"user_id"`
		FullName   string `json:"full_name"`
		TotalScore int    `json:"total_score"`
		ExamsTaken int    `json:"exams_taken"`
	}
	var entries []leaderboardEntry
	for rows.Next() {
		var e leaderboardEntry
		if err := rows.Scan(&e.UserID, &e.FullName, &e.TotalScore, &e.ExamsTaken); err == nil {
			entries = append(entries, e)
		}
	}
	if entries == nil {
		entries = []leaderboardEntry{}
	}
	c.JSON(http.StatusOK, entries)
}

// UserGetBatchMyResults returns the logged-in student's own exam results
// (MCQ live exams) for exams belonging to this batch.
func UserGetBatchMyResults(c *gin.Context) {
	mobile := c.GetString("mobile")
	userIDRaw, _ := c.Get("user_id")
	userID, ok := userIDRaw.(int)
	if !ok {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "user not found"})
		return
	}
	batchID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid batch id"})
		return
	}

	ctx := context.Background()
	if !userEnrolledInBatch(ctx, mobile, batchID) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "not enrolled in this batch"})
		return
	}

	var courseID int
	_ = database.DB.QueryRow(ctx, `SELECT COALESCE(course_id, 0) FROM batches WHERE id=$1`, batchID).Scan(&courseID)

	rows, err := database.DB.Query(ctx,
		`SELECT e.id, e.title, e.date, er.score, er.total_questions, er.completed_at::text
		 FROM exam_results er
		 JOIN exams e ON e.id = er.exam_id
		 WHERE er.user_id = $1 AND (e.batch_id = $2 OR (e.batch_id IS NULL AND e.course_id = $3))
		 ORDER BY er.completed_at DESC`, userID, batchID, courseID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	type myResult struct {
		ExamID         int    `json:"exam_id"`
		ExamTitle      string `json:"exam_title"`
		ExamDate       string `json:"exam_date"`
		Score          int    `json:"score"`
		TotalQuestions int    `json:"total_questions"`
		CompletedAt    string `json:"completed_at"`
	}
	var results []myResult
	for rows.Next() {
		var r myResult
		if err := rows.Scan(&r.ExamID, &r.ExamTitle, &r.ExamDate, &r.Score, &r.TotalQuestions, &r.CompletedAt); err == nil {
			results = append(results, r)
		}
	}
	if results == nil {
		results = []myResult{}
	}
	c.JSON(http.StatusOK, results)
}

// UserGetBatchStudents returns the roster of approved students in a batch
// the logged-in student is also enrolled in. Mobile numbers are withheld
// for privacy — only name/student-id are shown to fellow students.
func UserGetBatchStudents(c *gin.Context) {
	mobile := c.GetString("mobile")
	batchID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid batch id"})
		return
	}

	ctx := context.Background()
	if !userEnrolledInBatch(ctx, mobile, batchID) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "not enrolled in this batch"})
		return
	}

	rows, err := database.DB.Query(ctx,
		`SELECT e.id, e.full_name, COALESCE(e.student_id, '')
		 FROM enrollments e
		 WHERE e.batch_id = $1 AND e.status = 'approved'
		 ORDER BY e.full_name ASC`, batchID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	type studentInfo struct {
		ID        int    `json:"id"`
		FullName  string `json:"full_name"`
		StudentID string `json:"student_id"`
	}
	var students []studentInfo
	for rows.Next() {
		var s studentInfo
		if err := rows.Scan(&s.ID, &s.FullName, &s.StudentID); err == nil {
			students = append(students, s)
		}
	}
	if students == nil {
		students = []studentInfo{}
	}
	c.JSON(http.StatusOK, students)
}

// UserGetBatchNotices returns notices sent specifically to this batch.
func UserGetBatchNotices(c *gin.Context) {
	mobile := c.GetString("mobile")
	userIDRaw, _ := c.Get("user_id")
	userID, _ := userIDRaw.(int)
	batchID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid batch id"})
		return
	}

	ctx := context.Background()
	if !userEnrolledInBatch(ctx, mobile, batchID) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "not enrolled in this batch"})
		return
	}

	rows, err := database.DB.Query(ctx,
		`SELECT n.id, n.title, n.body, n.target, n.target_id, n.link_type, n.link_id, n.sent_at,
		        CASE WHEN nr.id IS NOT NULL THEN TRUE ELSE FALSE END as read_by_me
		 FROM notifications n
		 LEFT JOIN notification_reads nr ON nr.notification_id = n.id AND nr.user_id = $1
		 WHERE n.target = 'batch' AND n.target_id = $2
		 ORDER BY n.sent_at DESC`, userID, batchID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	var notices []models.NotificationWithRead
	for rows.Next() {
		var n models.NotificationWithRead
		if err := rows.Scan(&n.ID, &n.Title, &n.Body, &n.Target, &n.TargetID,
			&n.LinkType, &n.LinkID, &n.SentAt, &n.ReadByMe); err == nil {
			notices = append(notices, n)
		}
	}
	if notices == nil {
		notices = []models.NotificationWithRead{}
	}
	c.JSON(http.StatusOK, notices)
}

// UserGetBatchPayments returns this batch's monthly fee amount plus the
// logged-in student's own payment history for it.
func UserGetBatchPayments(c *gin.Context) {
	mobile := c.GetString("mobile")
	userIDRaw, _ := c.Get("user_id")
	userID, ok := userIDRaw.(int)
	if !ok {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "user not found"})
		return
	}
	batchID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid batch id"})
		return
	}

	ctx := context.Background()
	if !userEnrolledInBatch(ctx, mobile, batchID) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "not enrolled in this batch"})
		return
	}

	var monthlyFee int
	_ = database.DB.QueryRow(ctx, `SELECT COALESCE(monthly_fee, 0) FROM batches WHERE id=$1`, batchID).Scan(&monthlyFee)

	rows, err := database.DB.Query(ctx,
		`SELECT id, amount, method, COALESCE(transaction_id,''), status, COALESCE(receipt_number,''),
		        COALESCE(month,''), year, COALESCE(notes,''), created_at
		 FROM payments
		 WHERE user_id = $1 AND batch_id = $2
		 ORDER BY year DESC, created_at DESC`, userID, batchID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	type myPayment struct {
		ID            int     `json:"id"`
		Amount        float64 `json:"amount"`
		Method        string  `json:"method"`
		TransactionID string  `json:"transaction_id"`
		Status        string  `json:"status"`
		ReceiptNumber string  `json:"receipt_number"`
		Month         string  `json:"month"`
		Year          int     `json:"year"`
		Notes         string  `json:"notes"`
		CreatedAt     string  `json:"created_at"`
	}
	var payments []myPayment
	for rows.Next() {
		var p myPayment
		var createdAt time.Time
		if err := rows.Scan(&p.ID, &p.Amount, &p.Method, &p.TransactionID, &p.Status,
			&p.ReceiptNumber, &p.Month, &p.Year, &p.Notes, &createdAt); err == nil {
			p.CreatedAt = createdAt.Format("2006-01-02")
			payments = append(payments, p)
		}
	}
	if payments == nil {
		payments = []myPayment{}
	}

	c.JSON(http.StatusOK, gin.H{
		"monthly_fee": monthlyFee,
		"payments":    payments,
	})
}

// UserPayBatchFee lets the logged-in student submit a monthly fee payment
// for this batch (bKash/Nagad-style manual transfer). An admin verifies it
// afterwards — this just records it as 'pending'.
func UserPayBatchFee(c *gin.Context) {
	mobile := c.GetString("mobile")
	userIDRaw, _ := c.Get("user_id")
	userID, ok := userIDRaw.(int)
	if !ok {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "user not found"})
		return
	}
	batchID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid batch id"})
		return
	}

	ctx := context.Background()
	if !userEnrolledInBatch(ctx, mobile, batchID) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "not enrolled in this batch"})
		return
	}

	var req models.CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	receiptNo := fmt.Sprintf("EDU-%d-%04d", time.Now().Unix()%100000, time.Now().UnixNano()%10000)

	var p models.Payment
	err = database.DB.QueryRow(ctx,
		`INSERT INTO payments (user_id, batch_id, amount, method, transaction_id,
		 sender_number, receiver_number, receipt_number, month, year, notes)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING id, user_id, batch_id, amount, method, transaction_id, receipt_number, month, year, status, created_at`,
		userID, batchID, req.Amount, req.Method, req.TransactionID,
		req.SenderNumber, req.ReceiverNumber, receiptNo, req.Month, req.Year, req.Notes,
	).Scan(&p.ID, &p.UserID, &p.BatchID, &p.Amount, &p.Method, &p.TransactionID,
		&p.ReceiptNumber, &p.Month, &p.Year, &p.Status, &p.CreatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to submit payment"})
		return
	}
	c.JSON(http.StatusCreated, p)
}

// UserGetBatchLessons is the batch's study feed: class notes, videos and
// homework teachers have posted, newest first.
func UserGetBatchLessons(c *gin.Context) {
	mobile := c.GetString("mobile")
	batchID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid batch id"})
		return
	}

	ctx := context.Background()
	if !userEnrolledInBatch(ctx, mobile, batchID) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "not enrolled in this batch"})
		return
	}

	rows, err := database.DB.Query(ctx,
		`SELECT id, title, COALESCE(description, ''), COALESCE(subject, ''), COALESCE(chapter, ''), COALESCE(chapter_id, 0), COALESCE(topic, ''),
		        lesson_date::text, COALESCE(teacher_notes, ''), COALESCE(kind, 'lesson'), COALESCE(link_url, ''), created_at
		 FROM lessons
		 WHERE batch_id = $1
		 ORDER BY lesson_date DESC, created_at DESC
		 LIMIT 200`, batchID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	type lessonInfo struct {
		ID           int       `json:"id"`
		Title        string    `json:"title"`
		Description  string    `json:"description"`
		Subject      string    `json:"subject"`
		Chapter      string    `json:"chapter"`
		ChapterID    int       `json:"chapter_id"`
		Topic        string    `json:"topic"`
		LessonDate   string    `json:"lesson_date"`
		TeacherNotes string    `json:"teacher_notes"`
		Kind         string    `json:"kind"`
		LinkURL      string    `json:"link_url"`
		CreatedAt    time.Time `json:"created_at"`
	}
	lessons := []lessonInfo{}
	for rows.Next() {
		var l lessonInfo
		if err := rows.Scan(&l.ID, &l.Title, &l.Description, &l.Subject, &l.Chapter, &l.ChapterID, &l.Topic, &l.LessonDate,
			&l.TeacherNotes, &l.Kind, &l.LinkURL, &l.CreatedAt); err == nil {
			lessons = append(lessons, l)
		}
	}
	c.JSON(http.StatusOK, lessons)
}
