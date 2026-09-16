package handlers

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"edunova-server/database"
	"edunova-server/models"
)

func percentage(obtained, total float64) float64 {
	if total <= 0 {
		return 0
	}
	return (obtained / total) * 100
}

// AdminGetResults lists offline exam results, optionally filtered by student or batch.
func AdminGetResults(c *gin.Context) {
	userIDParam := c.Query("user_id")
	batchID := c.Query("batch_id")

	query := `SELECT r.id, r.user_id, u.full_name, COALESCE(u.student_class,''),
		 r.subject, r.exam_name, r.exam_date::text, r.marks_obtained, r.marks_total,
		 COALESCE(r.remarks,''), r.created_at
		 FROM student_results r
		 JOIN users u ON u.id = r.user_id`
	args := []interface{}{}
	var conditions []string
	if userIDParam != "" {
		args = append(args, userIDParam)
		conditions = append(conditions, fmt.Sprintf("r.user_id = $%d", len(args)))
	}
	if batchID != "" {
		args = append(args, batchID)
		conditions = append(conditions, fmt.Sprintf("r.user_id IN (SELECT e.user_id FROM enrollments e WHERE e.batch_id = $%d AND e.status = 'approved')", len(args)))
	}
	if teacherIDs, isTeacher := teacherAssignedBatchIDs(c); isTeacher {
		if len(teacherIDs) == 0 || (batchID != "" && !intSliceContains(teacherIDs, batchID)) {
			c.JSON(http.StatusOK, []models.StudentResult{})
			return
		}
		if batchID == "" {
			args = append(args, teacherIDs)
			conditions = append(conditions, fmt.Sprintf("r.user_id IN (SELECT e.user_id FROM enrollments e WHERE e.batch_id = ANY($%d) AND e.status = 'approved')", len(args)))
		}
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY r.exam_date DESC, r.id DESC"

	rows, err := database.DB.Query(context.Background(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	var results []models.StudentResult
	for rows.Next() {
		var r models.StudentResult
		if err := rows.Scan(&r.ID, &r.UserID, &r.StudentName, &r.StudentClass,
			&r.Subject, &r.ExamName, &r.ExamDate, &r.MarksObtained, &r.MarksTotal,
			&r.Remarks, &r.CreatedAt); err != nil {
			continue
		}
		r.Percentage = percentage(r.MarksObtained, r.MarksTotal)
		results = append(results, r)
	}
	if results == nil {
		results = []models.StudentResult{}
	}
	c.JSON(http.StatusOK, results)
}

func AdminCreateResult(c *gin.Context) {
	var req models.CreateResultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	if req.MarksObtained > req.MarksTotal {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "marks obtained cannot exceed total marks"})
		return
	}
	if !teacherCanUseStudentID(c, req.UserID) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "not authorized for this student"})
		return
	}

	adminEmail := c.GetString("admin_email")
	var createdBy interface{}
	if adminEmail != "" {
		var adminID int
		_ = database.DB.QueryRow(context.Background(),
			`SELECT id FROM admin_users WHERE email = $1`, adminEmail).Scan(&adminID)
		if adminID != 0 {
			createdBy = adminID
		}
	}

	var id int
	err := database.DB.QueryRow(context.Background(),
		`INSERT INTO student_results (user_id, subject, exam_name, exam_date, marks_obtained, marks_total, remarks, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
		req.UserID, req.Subject, req.ExamName, req.ExamDate, req.MarksObtained, req.MarksTotal, req.Remarks, createdBy,
	).Scan(&id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create result"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": id, "message": "result added"})
}

func AdminUpdateResult(c *gin.Context) {
	id := c.Param("id")
	var req models.CreateResultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	if req.MarksObtained > req.MarksTotal {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "marks obtained cannot exceed total marks"})
		return
	}
	if !teacherOwnsResult(c, id) || !teacherCanUseStudentID(c, req.UserID) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "not authorized for this student"})
		return
	}

	_, err := database.DB.Exec(context.Background(),
		`UPDATE student_results SET user_id=$1, subject=$2, exam_name=$3, exam_date=$4,
		 marks_obtained=$5, marks_total=$6, remarks=$7, updated_at=NOW() WHERE id=$8`,
		req.UserID, req.Subject, req.ExamName, req.ExamDate, req.MarksObtained, req.MarksTotal, req.Remarks, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update result"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "result updated"})
}

func AdminDeleteResult(c *gin.Context) {
	id := c.Param("id")
	if !teacherOwnsResult(c, id) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "not authorized for this student"})
		return
	}
	_, err := database.DB.Exec(context.Background(), `DELETE FROM student_results WHERE id=$1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to delete result"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "result deleted"})
}

// UserGetResults returns the logged-in student's raw result history.
func UserGetResults(c *gin.Context) {
	userIDRaw, _ := c.Get("user_id")
	userID, ok := userIDRaw.(int)
	if !ok {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "user not found"})
		return
	}

	rows, err := database.DB.Query(context.Background(),
		`SELECT id, user_id, subject, exam_name, exam_date::text, marks_obtained, marks_total, COALESCE(remarks,''), created_at
		 FROM student_results WHERE user_id = $1 ORDER BY exam_date DESC, id DESC`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	var results []models.StudentResult
	for rows.Next() {
		var r models.StudentResult
		if err := rows.Scan(&r.ID, &r.UserID, &r.Subject, &r.ExamName, &r.ExamDate,
			&r.MarksObtained, &r.MarksTotal, &r.Remarks, &r.CreatedAt); err != nil {
			continue
		}
		r.Percentage = percentage(r.MarksObtained, r.MarksTotal)
		results = append(results, r)
	}
	if results == nil {
		results = []models.StudentResult{}
	}
	c.JSON(http.StatusOK, results)
}

// UserGetResultSummary computes the guardian-facing progress view: overall
// percentage, a per-subject breakdown, and the weakest subjects as
// "areas of improvement" (average < 70%, worst first, capped at 3).
func UserGetResultSummary(c *gin.Context) {
	userIDRaw, _ := c.Get("user_id")
	userID, ok := userIDRaw.(int)
	if !ok {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "user not found"})
		return
	}

	rows, err := database.DB.Query(context.Background(),
		`SELECT id, subject, exam_name, exam_date::text, marks_obtained, marks_total, COALESCE(remarks,''), created_at
		 FROM student_results WHERE user_id = $1 ORDER BY exam_date DESC, id DESC`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	var all []models.StudentResult
	for rows.Next() {
		var r models.StudentResult
		if err := rows.Scan(&r.ID, &r.Subject, &r.ExamName, &r.ExamDate,
			&r.MarksObtained, &r.MarksTotal, &r.Remarks, &r.CreatedAt); err != nil {
			continue
		}
		r.UserID = userID
		r.Percentage = percentage(r.MarksObtained, r.MarksTotal)
		all = append(all, r)
	}

	summary := models.ResultSummary{
		BySubject:        []models.ResultSubjectSummary{},
		ImprovementAreas: []models.ResultSubjectSummary{},
		Recent:           []models.StudentResult{},
	}
	if len(all) == 0 {
		c.JSON(http.StatusOK, summary)
		return
	}

	summary.TotalExams = len(all)

	var totalObtained, totalMarks float64
	subjectTotals := map[string]*models.ResultSubjectSummary{}
	for _, r := range all {
		totalObtained += r.MarksObtained
		totalMarks += r.MarksTotal
		s, exists := subjectTotals[r.Subject]
		if !exists {
			s = &models.ResultSubjectSummary{Subject: r.Subject}
			subjectTotals[r.Subject] = s
		}
		s.AveragePercent += r.Percentage
		s.ExamCount++
	}
	summary.OverallPercent = percentage(totalObtained, totalMarks)

	for _, s := range subjectTotals {
		s.AveragePercent = s.AveragePercent / float64(s.ExamCount)
		summary.BySubject = append(summary.BySubject, *s)
	}
	sort.Slice(summary.BySubject, func(i, j int) bool {
		return summary.BySubject[i].Subject < summary.BySubject[j].Subject
	})

	weak := make([]models.ResultSubjectSummary, len(summary.BySubject))
	copy(weak, summary.BySubject)
	sort.Slice(weak, func(i, j int) bool { return weak[i].AveragePercent < weak[j].AveragePercent })
	for _, s := range weak {
		if s.AveragePercent < 70 && len(summary.ImprovementAreas) < 3 {
			summary.ImprovementAreas = append(summary.ImprovementAreas, s)
		}
	}

	recentCount := 10
	if len(all) < recentCount {
		recentCount = len(all)
	}
	summary.Recent = all[:recentCount]

	c.JSON(http.StatusOK, summary)
}
