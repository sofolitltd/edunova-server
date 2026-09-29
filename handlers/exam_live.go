package handlers

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"edunova-server/database"
	"edunova-server/models"
	"edunova-server/services"
)

func AdminToggleExamLive(c *gin.Context) {
	id := c.Param("id")
	if !teacherOwnsBatchRow(c, "exams", id) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "not authorized for this batch"})
		return
	}
	var req models.AdminToggleLiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var liveAt *time.Time
	if req.IsLive {
		now := time.Now()
		liveAt = &now
	}

	_, err := database.DB.Exec(context.Background(),
		`UPDATE exams SET is_live = $1, live_at = $2, updated_at = NOW() WHERE id = $3`,
		req.IsLive, liveAt, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update exam"})
		return
	}

	if req.IsLive {
		go sendExamLiveNotification(id)
	}

	c.JSON(http.StatusOK, gin.H{"message": "exam updated", "is_live": req.IsLive})
}

func sendExamLiveNotification(examID string) {
	var title, examTitle, classLevel string
	err := database.DB.QueryRow(context.Background(),
		`SELECT title, COALESCE(class_level, '') FROM exams WHERE id = $1`, examID,
	).Scan(&examTitle, &classLevel)
	if err != nil {
		return
	}
	title = "নতুন মডেল টেস্ট এসেছে!"
	body := examTitle + " — এখনই অংশ নিন!"

	linkData := map[string]string{
		"type":         "exam",
		"exam_id":      examID,
		"click_action": "FLUTTER_NOTIFICATION_CLICK",
	}

	_ = services.SendFCMTopicV1WithData("all", title, body, linkData)

	if classLevel != "" {
		topic := "class_" + classLevel
		_ = services.SendFCMTopicV1WithData(topic, title, body, linkData)
	}

	_, _ = database.DB.Exec(context.Background(),
		`INSERT INTO notifications (title, body, target, target_id, link_type, link_id) VALUES ($1, $2, 'exam', $3, 'exam', $3)`,
		title, body, examID)
}

func UserGetLiveExams(c *gin.Context) {
	rows, err := database.DB.Query(context.Background(),
		`SELECT e.id, e.title, COALESCE(e.course_id, 0), COALESCE(c.title,''), COALESCE(e.date,''), COALESCE(e.time,''),
		 COALESCE(e.duration,''), e.total_questions, COALESCE(e.description,''), COALESCE(e.class_level,''),
		 e.is_live, e.live_at, e.created_at, e.updated_at
		 FROM exams e
		 LEFT JOIN courses c ON e.course_id = c.id
		 WHERE e.is_live = TRUE
		 ORDER BY e.live_at DESC`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	type LiveExam struct {
		ID             int     `json:"id"`
		Title          string  `json:"title"`
		CourseID       int     `json:"course_id"`
		CourseName     string  `json:"course_name"`
		Date           string  `json:"date"`
		Time           string  `json:"time"`
		Duration       string  `json:"duration"`
		TotalQuestions int     `json:"total_questions"`
		Description    string  `json:"description"`
		ClassLevel     string  `json:"class_level"`
		IsLive         bool    `json:"is_live"`
		LiveAt         *string `json:"live_at"`
		CreatedAt      string  `json:"created_at"`
		UpdatedAt      string  `json:"updated_at"`
	}

	var exams []LiveExam
	for rows.Next() {
		var e LiveExam
		if err := rows.Scan(&e.ID, &e.Title, &e.CourseID, &e.CourseName, &e.Date, &e.Time,
			&e.Duration, &e.TotalQuestions, &e.Description, &e.ClassLevel,
			&e.IsLive, &e.LiveAt, &e.CreatedAt, &e.UpdatedAt); err != nil {
			log.Printf("Error scanning live exam: %v", err)
			continue
		}
		exams = append(exams, e)
	}
	if exams == nil {
		exams = []LiveExam{}
	}
	c.JSON(http.StatusOK, exams)
}

func UserSubmitExamResult(c *gin.Context) {
	userIDRaw, _ := c.Get("user_id")
	userID := userIDRaw.(int)
	examID := c.Param("id")

	var req struct {
		Score          int `json:"score"`
		TotalQuestions int `json:"total_questions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var exists bool
	_ = database.DB.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM exam_results WHERE exam_id = $1 AND user_id = $2)`,
		examID, userID).Scan(&exists)

	if exists {
		_, err := database.DB.Exec(context.Background(),
			`UPDATE exam_results SET score = $1, total_questions = $2, completed_at = NOW()
			 WHERE exam_id = $3 AND user_id = $4`,
			req.Score, req.TotalQuestions, examID, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update result"})
			return
		}
	} else {
		_, err := database.DB.Exec(context.Background(),
			`INSERT INTO exam_results (exam_id, user_id, score, total_questions) VALUES ($1, $2, $3, $4)`,
			examID, userID, req.Score, req.TotalQuestions)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to save result"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "result saved"})
}

func UserGetExamResults(c *gin.Context) {
	userIDRaw, _ := c.Get("user_id")
	userID := userIDRaw.(int)
	rows, err := database.DB.Query(context.Background(),
		`SELECT er.id, er.exam_id, COALESCE(e.title,''), er.user_id, er.score, er.total_questions, er.completed_at
		 FROM exam_results er
		 LEFT JOIN exams e ON er.exam_id = e.id
		 WHERE er.user_id = $1
		 ORDER BY er.completed_at DESC`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	var results []models.ExamResult
	for rows.Next() {
		var r models.ExamResult
		if err := rows.Scan(&r.ID, &r.ExamID, &r.ExamTitle, &r.UserID, &r.Score, &r.TotalQuestions, &r.CompletedAt); err != nil {
			continue
		}
		results = append(results, r)
	}
	if results == nil {
		results = []models.ExamResult{}
	}
	c.JSON(http.StatusOK, results)
}
