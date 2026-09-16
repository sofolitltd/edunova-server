package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"edunova-server/database"
	"edunova-server/models"
	"edunova-server/services"
)

func AdminGetStudents(c *gin.Context) {
	batchID := c.Query("batch_id")

	query := `SELECT DISTINCT u.id, u.full_name, u.mobile, u.verified,
		 COALESCE(u.father_name,''), COALESCE(u.father_mobile,''),
		 COALESCE(u.mother_name,''), COALESCE(u.mother_mobile,''),
		 COALESCE(u.notification_mobile,''), COALESCE(u.student_class,''),
		 COALESCE(u.shift,''), COALESCE(u.school,''),
		 u.created_at, u.updated_at
		 FROM users u`
	var args []interface{}
	var conditions []string

	if teacherIDs, isTeacher := teacherAssignedBatchIDs(c); isTeacher {
		if len(teacherIDs) == 0 || (batchID != "" && !intSliceContains(teacherIDs, batchID)) {
			c.JSON(http.StatusOK, []models.User{})
			return
		}
		if batchID != "" {
			args = append(args, batchID)
			conditions = append(conditions, fmt.Sprintf("u.id IN (SELECT e.user_id FROM enrollments e WHERE e.batch_id = $%d AND e.status = 'approved')", len(args)))
		} else {
			args = append(args, teacherIDs)
			conditions = append(conditions, fmt.Sprintf("u.id IN (SELECT e.user_id FROM enrollments e WHERE e.batch_id = ANY($%d) AND e.status = 'approved')", len(args)))
		}
	} else if batchID != "" {
		args = append(args, batchID)
		conditions = append(conditions, fmt.Sprintf("u.id IN (SELECT e.user_id FROM enrollments e WHERE e.batch_id = $%d AND e.status = 'approved')", len(args)))
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY u.full_name"

	rows, err := database.DB.Query(context.Background(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch students"})
		return
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.FullName, &u.Mobile, &u.Verified,
			&u.FatherName, &u.FatherMobile,
			&u.MotherName, &u.MotherMobile,
			&u.NotificationMobile, &u.StudentClass,
			&u.Shift, &u.School,
			&u.CreatedAt, &u.UpdatedAt); err != nil {
			continue
		}
		users = append(users, u)
	}
	if users == nil {
		users = []models.User{}
	}
	c.JSON(http.StatusOK, users)
}

func AdminMarkAttendance(c *gin.Context) {
	var req models.BulkMarkAttendanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	adminID, _ := c.Get("admin_id")
	var markedBy interface{}
	if id, ok := adminID.(int); ok {
		markedBy = id
	}

	var imported, skipped int
	ctx := context.Background()

	for _, entry := range req.Entries {
		if !teacherCanUseStudentID(c, entry.StudentID) {
			skipped++
			continue
		}
		_, err := database.DB.Exec(ctx,
			`INSERT INTO attendance (student_id, date, status, marked_by, notes)
			 VALUES ($1, $2, $3, $4, $5)
			 ON CONFLICT (student_id, date) DO UPDATE SET status=$3, marked_by=$4, notes=$5`,
			entry.StudentID, req.Date, entry.Status, markedBy, entry.Notes)
		if err != nil {
			skipped++
			continue
		}
		imported++

		// Notify the guardian/student's own account — there is no separate
		// guardian account to look up, the same login is used by both.
		go sendAttendanceNotification(entry.StudentID, req.Date, entry.Status)
	}

	c.JSON(http.StatusOK, gin.H{
		"imported": imported,
		"skipped":  skipped,
		"message":  fmt.Sprintf("Marked %d students, %d skipped", imported, skipped),
	})
}

func AdminGetAttendance(c *gin.Context) {
	date := c.Query("date")
	batchID := c.Query("batch_id")
	studentID := c.Query("student_id")

	if date == "" && studentID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "date is required"})
		return
	}

	query := `
		SELECT a.id, a.student_id, u.full_name, a.date::text, a.status, a.marked_by,
		 COALESCE(au.full_name, ''), COALESCE(a.notes, ''), a.created_at
		FROM attendance a
		JOIN users u ON a.student_id = u.id
		LEFT JOIN admin_users au ON a.marked_by = au.id`
	conditions := []string{}
	args := []interface{}{}

	if date != "" {
		args = append(args, date)
		conditions = append(conditions, `a.date = $`+strconv.Itoa(len(args)))
	}
	if studentID != "" {
		args = append(args, studentID)
		conditions = append(conditions, `a.student_id = $`+strconv.Itoa(len(args)))
	}
	if batchID != "" {
		args = append(args, batchID)
		conditions = append(conditions, `a.student_id IN (
			SELECT e.user_id FROM enrollments e WHERE e.batch_id = $`+strconv.Itoa(len(args))+` AND e.status = 'approved'
		)`)
	}
	if teacherIDs, isTeacher := teacherAssignedBatchIDs(c); isTeacher {
		if len(teacherIDs) == 0 || (batchID != "" && !intSliceContains(teacherIDs, batchID)) {
			c.JSON(http.StatusOK, []models.Attendance{})
			return
		}
		if batchID == "" {
			args = append(args, teacherIDs)
			conditions = append(conditions, `a.student_id IN (
				SELECT e.user_id FROM enrollments e WHERE e.batch_id = ANY($`+strconv.Itoa(len(args))+`) AND e.status = 'approved'
			)`)
		}
	}
	if len(conditions) > 0 {
		query += ` WHERE ` + strings.Join(conditions, " AND ")
	}

	query += ` ORDER BY a.date DESC, u.full_name`

	rows, err := database.DB.Query(context.Background(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch attendance"})
		return
	}
	defer rows.Close()

	var records []models.Attendance
	for rows.Next() {
		var a models.Attendance
		if err := rows.Scan(&a.ID, &a.StudentID, &a.StudentName, &a.Date, &a.Status,
			&a.MarkedBy, &a.MarkedByName, &a.Notes, &a.CreatedAt); err != nil {
			continue
		}
		records = append(records, a)
	}
	if records == nil {
		records = []models.Attendance{}
	}
	c.JSON(http.StatusOK, records)
}

// UserGetMyAttendance returns the logged-in student's own attendance history
// (guardian-facing — there is no separate guardian account, so "my" here
// means whoever is logged into this student's account), most recent first.
func UserGetMyAttendance(c *gin.Context) {
	userIDRaw, _ := c.Get("user_id")
	userID, ok := userIDRaw.(int)
	if !ok {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "user not found"})
		return
	}

	rows, err := database.DB.Query(context.Background(),
		`SELECT id, date::text, status, COALESCE(notes,'')
		 FROM attendance WHERE student_id = $1 ORDER BY date DESC LIMIT 90`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	type AttendanceRecord struct {
		ID     int    `json:"id"`
		Date   string `json:"date"`
		Status string `json:"status"`
		Notes  string `json:"notes"`
	}

	var present, absent, late int
	records := []AttendanceRecord{}
	for rows.Next() {
		var r AttendanceRecord
		if err := rows.Scan(&r.ID, &r.Date, &r.Status, &r.Notes); err != nil {
			continue
		}
		switch r.Status {
		case "present":
			present++
		case "absent":
			absent++
		case "late":
			late++
		}
		records = append(records, r)
	}

	total := present + absent + late
	rate := 0.0
	if total > 0 {
		rate = float64(present) / float64(total) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"records":         records,
		"total_days":      total,
		"present":         present,
		"absent":          absent,
		"late":            late,
		"attendance_rate": rate,
	})
}

func AdminGetAttendanceReport(c *gin.Context) {
	date := c.Query("date")
	if date == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "date is required"})
		return
	}
	batchID := c.Query("batch_id")

	var total, present, absent, late int
	ctx := context.Background()

	teacherIDs, isTeacher := teacherAssignedBatchIDs(c)
	if isTeacher && (len(teacherIDs) == 0 || (batchID != "" && !intSliceContains(teacherIDs, batchID))) {
		c.JSON(http.StatusOK, models.AttendanceReport{Date: date})
		return
	}

	if batchID != "" {
		batchFilter := `AND student_id IN (
			SELECT e.user_id FROM enrollments e WHERE e.batch_id = $2 AND e.status = 'approved'
		)`
		database.DB.QueryRow(ctx, `SELECT COUNT(*) FROM enrollments WHERE batch_id=$1 AND status='approved'`, batchID).Scan(&total)
		database.DB.QueryRow(ctx, `SELECT COUNT(*) FROM attendance WHERE date=$1 AND status='present' `+batchFilter, date, batchID).Scan(&present)
		database.DB.QueryRow(ctx, `SELECT COUNT(*) FROM attendance WHERE date=$1 AND status='absent' `+batchFilter, date, batchID).Scan(&absent)
		database.DB.QueryRow(ctx, `SELECT COUNT(*) FROM attendance WHERE date=$1 AND status='late' `+batchFilter, date, batchID).Scan(&late)
	} else if isTeacher {
		batchFilter := `AND student_id IN (
			SELECT e.user_id FROM enrollments e WHERE e.batch_id = ANY($2) AND e.status = 'approved'
		)`
		database.DB.QueryRow(ctx, `SELECT COUNT(*) FROM enrollments WHERE batch_id = ANY($1) AND status='approved'`, teacherIDs).Scan(&total)
		database.DB.QueryRow(ctx, `SELECT COUNT(*) FROM attendance WHERE date=$1 AND status='present' `+batchFilter, date, teacherIDs).Scan(&present)
		database.DB.QueryRow(ctx, `SELECT COUNT(*) FROM attendance WHERE date=$1 AND status='absent' `+batchFilter, date, teacherIDs).Scan(&absent)
		database.DB.QueryRow(ctx, `SELECT COUNT(*) FROM attendance WHERE date=$1 AND status='late' `+batchFilter, date, teacherIDs).Scan(&late)
	} else {
		database.DB.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE verified=true`).Scan(&total)
		database.DB.QueryRow(ctx, `SELECT COUNT(*) FROM attendance WHERE date=$1 AND status='present'`, date).Scan(&present)
		database.DB.QueryRow(ctx, `SELECT COUNT(*) FROM attendance WHERE date=$1 AND status='absent'`, date).Scan(&absent)
		database.DB.QueryRow(ctx, `SELECT COUNT(*) FROM attendance WHERE date=$1 AND status='late'`, date).Scan(&late)
	}

	rate := 0.0
	if total > 0 {
		rate = float64(present) / float64(total) * 100
	}

	c.JSON(http.StatusOK, models.AttendanceReport{
		Date:           date,
		Total:          total,
		Present:        present,
		Absent:         absent,
		Late:           late,
		AttendanceRate: rate,
	})
}

// AdminGetBatchMonthlyAttendance returns a batch-scoped attendance summary
// for a given month: per-day rollups (for a calendar-style view) and
// per-student totals/rates, so admins can see monthly trends rather than
// just a single day.
func AdminGetBatchMonthlyAttendance(c *gin.Context) {
	batchID := c.Param("id")
	month := c.Query("month")
	if month == "" {
		month = time.Now().Format("2006-01")
	}

	ctx := context.Background()

	dayRows, err := database.DB.Query(ctx,
		`SELECT a.date::text,
			COUNT(*) FILTER (WHERE a.status='present') AS present,
			COUNT(*) FILTER (WHERE a.status='absent') AS absent,
			COUNT(*) FILTER (WHERE a.status='late') AS late
		 FROM attendance a
		 WHERE to_char(a.date, 'YYYY-MM') = $1
		 AND a.student_id IN (
			SELECT e.user_id FROM enrollments e WHERE e.batch_id = $2 AND e.status = 'approved'
		 )
		 GROUP BY a.date
		 ORDER BY a.date`,
		month, batchID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch monthly attendance"})
		return
	}
	defer dayRows.Close()

	var days []models.BatchAttendanceDay
	for dayRows.Next() {
		var d models.BatchAttendanceDay
		if err := dayRows.Scan(&d.Date, &d.Present, &d.Absent, &d.Late); err == nil {
			days = append(days, d)
		}
	}
	if days == nil {
		days = []models.BatchAttendanceDay{}
	}
	totalClasses := len(days)

	studentRows, err := database.DB.Query(ctx,
		`SELECT u.id, u.full_name,
			COUNT(*) FILTER (WHERE a.status='present') AS present,
			COUNT(*) FILTER (WHERE a.status='absent') AS absent,
			COUNT(*) FILTER (WHERE a.status='late') AS late
		 FROM enrollments e
		 JOIN users u ON u.id = e.user_id
		 LEFT JOIN attendance a ON a.student_id = u.id AND to_char(a.date, 'YYYY-MM') = $1
		 WHERE e.batch_id = $2 AND e.status = 'approved'
		 GROUP BY u.id, u.full_name
		 ORDER BY u.full_name`,
		month, batchID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch monthly attendance"})
		return
	}
	defer studentRows.Close()

	var students []models.BatchStudentAttendance
	var totalPresent int
	for studentRows.Next() {
		var s models.BatchStudentAttendance
		if err := studentRows.Scan(&s.StudentID, &s.FullName, &s.Present, &s.Absent, &s.Late); err != nil {
			continue
		}
		if totalClasses > 0 {
			s.Rate = float64(s.Present) / float64(totalClasses) * 100
		}
		totalPresent += s.Present
		students = append(students, s)
	}
	if students == nil {
		students = []models.BatchStudentAttendance{}
	}

	overallRate := 0.0
	if len(students) > 0 && totalClasses > 0 {
		overallRate = float64(totalPresent) / float64(len(students)*totalClasses) * 100
	}

	c.JSON(http.StatusOK, models.BatchMonthlyAttendance{
		Month:        month,
		TotalClasses: totalClasses,
		OverallRate:  overallRate,
		Days:         days,
		Students:     students,
	})
}

func AdminGetHolidays(c *gin.Context) {
	rows, err := database.DB.Query(context.Background(),
		`SELECT id, date::text, reason, created_at FROM holidays ORDER BY date DESC`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch holidays"})
		return
	}
	defer rows.Close()

	var holidays []models.Holiday
	for rows.Next() {
		var h models.Holiday
		if err := rows.Scan(&h.ID, &h.Date, &h.Reason, &h.CreatedAt); err != nil {
			continue
		}
		holidays = append(holidays, h)
	}
	if holidays == nil {
		holidays = []models.Holiday{}
	}
	c.JSON(http.StatusOK, holidays)
}

func AdminCreateHoliday(c *gin.Context) {
	var req models.CreateHolidayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var h models.Holiday
	err := database.DB.QueryRow(context.Background(),
		`INSERT INTO holidays (date, reason) VALUES ($1, $2)
		 RETURNING id, date::text, reason, created_at`,
		req.Date, req.Reason,
	).Scan(&h.ID, &h.Date, &h.Reason, &h.CreatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create holiday"})
		return
	}
	c.JSON(http.StatusCreated, h)
}

func AdminDeleteHoliday(c *gin.Context) {
	id := c.Param("id")
	_, err := database.DB.Exec(context.Background(), `DELETE FROM holidays WHERE id=$1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to delete holiday"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "holiday deleted"})
}

func RegisterDeviceToken(c *gin.Context) {
	mobile := c.GetString("mobile")

	var req models.RegisterDeviceTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	platform := req.Platform
	if platform == "" {
		platform = "android"
	}

	var userID int
	err := database.DB.QueryRow(context.Background(), `SELECT id FROM users WHERE mobile=$1`, mobile).Scan(&userID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "user not found"})
		return
	}

	_, err = database.DB.Exec(context.Background(),
		`INSERT INTO device_tokens (user_id, token, platform) VALUES ($1, $2, $3)
		 ON CONFLICT (user_id, token) DO NOTHING`,
		userID, req.Token, platform)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to register device token"})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{Message: "device token registered"})
}

var attendanceStatusLabel = map[string]string{
	"present": "present",
	"absent":  "absent",
	"late":    "late",
}

// sendAttendanceNotification pushes an attendance update to the student's
// own account. There is no separate guardian account (guardians use the
// same login as the student), so this notifies that account's device
// tokens directly rather than trying to resolve a distinct "parent user".
func sendAttendanceNotification(studentID int, date string, status string) {
	var studentName string
	ctx := context.Background()
	err := database.DB.QueryRow(ctx,
		`SELECT full_name FROM users WHERE id=$1`, studentID,
	).Scan(&studentName)
	if err != nil {
		return
	}

	var tokens []string
	rows, _ := database.DB.Query(ctx, `SELECT token FROM device_tokens WHERE user_id=$1`, studentID)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var token string
			if rows.Scan(&token) == nil {
				tokens = append(tokens, token)
			}
		}
	}

	statusLabel := attendanceStatusLabel[status]
	if statusLabel == "" {
		statusLabel = status
	}

	title := "EduNova Attendance"
	body := fmt.Sprintf("%s was marked %s on %s", studentName, statusLabel, date)

	for _, token := range tokens {
		if err := services.SendFCMV1(token, title, body); err != nil {
			log.Printf("[FCM] Failed to send attendance notification: %v", err)
		}
	}
}
