package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"edunova-server/database"
	"edunova-server/models"
	"edunova-server/services"
)

func UserSubmitTransition(c *gin.Context) {
	userIDRaw, _ := c.Get("user_id")
	userID := userIDRaw.(int)
	var req models.CreateTransitionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var id int
	err := database.DB.QueryRow(context.Background(),
		`INSERT INTO student_transitions (user_id, from_class, to_class, gpa, subjects, result_notes)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		userID, req.FromClass, req.ToClass, req.GPA, req.Subjects, req.ResultNotes).Scan(&id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to submit transition"})
		return
	}

	go generateFeedback(id, userID, req.GPA)

	c.JSON(http.StatusCreated, gin.H{"id": id, "message": "transition submitted"})
}

func generateFeedback(transitionID, userID int, gpa float64) {
	var userName string
	_ = database.DB.QueryRow(context.Background(),
		`SELECT full_name FROM users WHERE id = $1`, userID).Scan(&userName)

	var feedbackType, title, message string
	var guidelines []string

	if gpa >= 5.0 {
		feedbackType = "high_performer"
		title = "অভিনন্দন! চমৎকার ফলাফল!"
		message = fmt.Sprintf("প্রিয় %s, আপনার GPA %.1f অত্যন্ত ভালো। আপনি অসাধারণ একজন শিক্ষার্থী!", userName, gpa)
		guidelines = []string{
			"আপনার জ্ঞানকে আরও গভীর করুন",
			"অ্যাডভান্সড বই ও রিসোর্স অনুসন্ধান করুন",
			"বিষয়গত প্রতিযোগিতায় অংশ নিন",
			"পিয়ার টিচিং করুন — অন্যদের শেখানো নিজেকে আরও শক্তিশালী করে",
			"ভবিষ্যতের জন্য প্রস্তুতি নিন — বোর্ড পরীক্ষা, প্রবেশিকা, বৃত্তি",
		}
	} else if gpa >= 3.5 {
		feedbackType = "good"
		title = "ভালো ফলাফল! আরও এগিয়ে যান!"
		message = fmt.Sprintf("প্রিয় %s, আপনার GPA %.1f ভালো। কিছু বিষয়ে আরও উন্নতি সম্ভব!", userName, gpa)
		guidelines = []string{
			"দুর্বল বিষয়গুলো চিহ্নিত করুন",
			"প্রতিদিন ৩০ মিনিট অতিরিক্ত অনুশীলন করুন",
			"শিক্ষকদের সাথে ব্যক্তিগত কথা বলুন",
			"মক পরীক্ষা দিন",
			"পড়ার সময়সূচি তৈরি করুন",
		}
	} else if gpa >= 2.0 {
		feedbackType = "needs_improvement"
		title = "উন্নতির সুযোগ আছে!"
		message = fmt.Sprintf("প্রিয় %s, আপনার GPA %.1f। নিয়মিত অনুশীলন করলে অনেক ভালো ফলাফল পাবেন।", userName, gpa)
		guidelines = []string{
			"প্রতিদিন নিয়মিত পড়ুন — কমপক্ষে ২ ঘণ্টা",
			"দুর্বল বিষয়ে বেশি সময় দিন",
			"বিস্তারিত নোট তৈরি করুন",
			"প্রশ্নের উত্তর লিখে অনুশীলন করুন",
			"শিক্ষকের সাহায্য নিন",
		}
	} else {
		feedbackType = "struggling"
		title = "আমরা আপনার পাশে আছি!"
		message = fmt.Sprintf("প্রিয় %s, চিন্তা করবেন না। আমাদের টিম আপনাকে সাহায্য করবে।", userName)
		guidelines = []string{
			"প্রতিদিন কমপক্ষে ১ ঘণ্টা পড়ুন",
			"মৌলিক বিষয়গুলো থেকে শুরু করুন",
			"ভিডিও লেকচার দেখুন",
			"ছোট ছোট লক্ষ্য ঠিক করুন",
			"নিয়মিত পরীক্ষা দিন",
		}
	}

	guidelinesJSON, _ := json.Marshal(guidelines)

	_, err := database.DB.Exec(context.Background(),
		`INSERT INTO student_feedback (transition_id, user_id, feedback_type, title, message, guidelines)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		transitionID, userID, feedbackType, title, message, guidelinesJSON)
	if err != nil {
		log.Printf("Error creating feedback: %v", err)
		return
	}

	go func() {
		var fcmToken string
		_ = database.DB.QueryRow(context.Background(),
			`SELECT token FROM device_tokens WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`,
			userID).Scan(&fcmToken)
		if fcmToken != "" {
			_ = services.SendFCMV1(fcmToken, title, message)
		}
	}()
}

func UserGetMyTransitions(c *gin.Context) {
	userIDRaw, _ := c.Get("user_id")
	userID := userIDRaw.(int)
	rows, err := database.DB.Query(context.Background(),
		`SELECT id, user_id, from_class, to_class, gpa, subjects, result_notes, submitted_at
		 FROM student_transitions WHERE user_id = $1 ORDER BY submitted_at DESC`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	var transitions []models.StudentTransition
	for rows.Next() {
		var t models.StudentTransition
		if err := rows.Scan(&t.ID, &t.UserID, &t.FromClass, &t.ToClass, &t.GPA,
			&t.Subjects, &t.ResultNotes, &t.SubmittedAt); err != nil {
			continue
		}
		transitions = append(transitions, t)
	}
	if transitions == nil {
		transitions = []models.StudentTransition{}
	}
	c.JSON(http.StatusOK, transitions)
}

func UserGetMyFeedback(c *gin.Context) {
	userIDRaw, _ := c.Get("user_id")
	userID := userIDRaw.(int)
	rows, err := database.DB.Query(context.Background(),
		`SELECT id, transition_id, user_id, feedback_type, title, message, guidelines, created_at
		 FROM student_feedback WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	var feedbacks []models.StudentFeedback
	for rows.Next() {
		var f models.StudentFeedback
		if err := rows.Scan(&f.ID, &f.TransitionID, &f.UserID, &f.FeedbackType,
			&f.Title, &f.Message, &f.Guidelines, &f.CreatedAt); err != nil {
			continue
		}
		feedbacks = append(feedbacks, f)
	}
	if feedbacks == nil {
		feedbacks = []models.StudentFeedback{}
	}
	c.JSON(http.StatusOK, feedbacks)
}

func AdminGetTransitions(c *gin.Context) {
	rows, err := database.DB.Query(context.Background(),
		`SELECT st.id, st.user_id, COALESCE(u.full_name,''), st.from_class, st.to_class, st.gpa, st.subjects, st.result_notes, st.submitted_at
		 FROM student_transitions st
		 LEFT JOIN users u ON st.user_id = u.id
		 ORDER BY st.submitted_at DESC`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	var transitions []models.StudentTransition
	for rows.Next() {
		var t models.StudentTransition
		if err := rows.Scan(&t.ID, &t.UserID, &t.UserName, &t.FromClass, &t.ToClass,
			&t.GPA, &t.Subjects, &t.ResultNotes, &t.SubmittedAt); err != nil {
			continue
		}
		transitions = append(transitions, t)
	}
	if transitions == nil {
		transitions = []models.StudentTransition{}
	}
	c.JSON(http.StatusOK, transitions)
}
