package handlers

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"edunova-server/database"
	"edunova-server/models"
)

func AdminGetDailyContent(c *gin.Context) {
	contentType := c.Query("content_type")
	classLevel := c.Query("class_level")

	query := `SELECT id, content_type, class_level, title, body, answer, language, is_published, created_by, created_at
		 FROM daily_content WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if contentType != "" {
		query += " AND content_type = $" + strconv.Itoa(argIdx)
		args = append(args, contentType)
		argIdx++
	}
	if classLevel != "" {
		query += " AND class_level = $" + strconv.Itoa(argIdx)
		args = append(args, classLevel)
		argIdx++
	}
	query += " ORDER BY created_at DESC"

	rows, err := database.DB.Query(context.Background(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	var items []models.DailyContent
	for rows.Next() {
		var d models.DailyContent
		if err := rows.Scan(&d.ID, &d.ContentType, &d.ClassLevel, &d.Title, &d.Body,
			&d.Answer, &d.Language, &d.IsPublished, &d.CreatedBy, &d.CreatedAt); err != nil {
			continue
		}
		items = append(items, d)
	}
	if items == nil {
		items = []models.DailyContent{}
	}
	c.JSON(http.StatusOK, items)
}

func AdminCreateDailyContent(c *gin.Context) {
	var req models.CreateDailyContentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
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

	if req.Language == "" {
		req.Language = "bn"
	}

	var id int
	err := database.DB.QueryRow(context.Background(),
		`INSERT INTO daily_content (content_type, class_level, title, body, answer, language, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		req.ContentType, req.ClassLevel, req.Title, req.Body, req.Answer, req.Language, createdBy).Scan(&id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create content"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": id, "message": "content created"})
}

func AdminUpdateDailyContent(c *gin.Context) {
	id := c.Param("id")
	var req models.CreateDailyContentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	if req.Language == "" {
		req.Language = "bn"
	}

	_, err := database.DB.Exec(context.Background(),
		`UPDATE daily_content SET content_type=$1, class_level=$2, title=$3, body=$4, answer=$5, language=$6
		 WHERE id=$7`,
		req.ContentType, req.ClassLevel, req.Title, req.Body, req.Answer, req.Language, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update content"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "content updated"})
}

func AdminDeleteDailyContent(c *gin.Context) {
	id := c.Param("id")
	_, err := database.DB.Exec(context.Background(), `DELETE FROM daily_content WHERE id=$1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to delete content"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "content deleted"})
}

func UserGetDailyContent(c *gin.Context) {
	userIDRaw, _ := c.Get("user_id")
	userID := userIDRaw.(int)

	var classLevel string
	_ = database.DB.QueryRow(context.Background(),
		`SELECT COALESCE(student_class, '') FROM users WHERE id = $1`, userID).Scan(&classLevel)

	today := time.Now().Format("2006-01-02")
	contentTypeFilter := c.Query("content_type")

	type ContentItem struct {
		ID          int    `json:"id"`
		ContentType string `json:"content_type"`
		Title       string `json:"title"`
		Body        string `json:"body"`
		Answer      string `json:"answer"`
		DeliveredAt string `json:"delivered_at"`
		IsReviewed  bool   `json:"is_reviewed"`
	}

	items := []ContentItem{}

	if contentTypeFilter != "" {
		// Browsing one content type: return every matching published item, not just one.
		rows, err := database.DB.Query(context.Background(),
			`SELECT dc.id, dc.content_type, dc.title, dc.body, dc.answer
			 FROM daily_content dc
			 WHERE dc.content_type = $1 AND dc.is_published = TRUE
			   AND (dc.class_level = $2 OR dc.class_level = '')
			 ORDER BY dc.created_at DESC`, contentTypeFilter, classLevel,
		)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var item ContentItem
				if err := rows.Scan(&item.ID, &item.ContentType, &item.Title, &item.Body, &item.Answer); err != nil {
					continue
				}
				item.DeliveredAt = today
				items = append(items, item)
			}
		}
	} else {
		// "All": one random pick per content type, as today's daily set.
		contentTypes := []string{"vocabulary", "math", "science", "news"}
		for _, ct := range contentTypes {
			var item ContentItem
			err := database.DB.QueryRow(context.Background(),
				`SELECT dc.id, dc.content_type, dc.title, dc.body, dc.answer
				 FROM daily_content dc
				 WHERE dc.content_type = $1 AND dc.is_published = TRUE
				   AND (dc.class_level = $2 OR dc.class_level = '')
				 ORDER BY RANDOM() LIMIT 1`, ct, classLevel,
			).Scan(&item.ID, &item.ContentType, &item.Title, &item.Body, &item.Answer)

			if err != nil {
				continue
			}

			_, _ = database.DB.Exec(context.Background(),
				`INSERT INTO daily_content_delivery (content_id, user_id, delivered_at)
				 VALUES ($1, $2, NOW())
				 ON CONFLICT (content_id, user_id) DO NOTHING`,
				item.ID, userID)

			item.DeliveredAt = today
			items = append(items, item)
		}
	}

	c.JSON(http.StatusOK, items)
}

func UserGetRevisionContent(c *gin.Context) {
	userIDRaw, _ := c.Get("user_id")
	userID := userIDRaw.(int)

	rows, err := database.DB.Query(context.Background(),
		`SELECT dc.id, dc.content_type, dc.title, dc.body, dc.answer, dcd.next_review_at, dcd.review_count
		 FROM daily_content_delivery dcd
		 JOIN daily_content dc ON dcd.content_id = dc.id
		 WHERE dcd.user_id = $1 AND dcd.next_review_at <= NOW() AND dc.is_published = TRUE
		 ORDER BY dcd.next_review_at ASC
		 LIMIT 10`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	type RevisionItem struct {
		ID          int    `json:"id"`
		ContentType string `json:"content_type"`
		Title       string `json:"title"`
		Body        string `json:"body"`
		Answer      string `json:"answer"`
		ReviewCount int    `json:"review_count"`
	}

	var items []RevisionItem
	for rows.Next() {
		var r RevisionItem
		var nextReview interface{}
		if err := rows.Scan(&r.ID, &r.ContentType, &r.Title, &r.Body, &r.Answer, &nextReview, &r.ReviewCount); err != nil {
			log.Printf("Error scanning revision: %v", err)
			continue
		}
		items = append(items, r)
	}
	if items == nil {
		items = []RevisionItem{}
	}
	c.JSON(http.StatusOK, items)
}

func UserMarkContentViewed(c *gin.Context) {
	userIDRaw, _ := c.Get("user_id")
	userID := userIDRaw.(int)
	contentID := c.Param("id")

	var reviewCount int
	_ = database.DB.QueryRow(context.Background(),
		`SELECT review_count FROM daily_content_delivery WHERE content_id=$1 AND user_id=$2`,
		contentID, userID).Scan(&reviewCount)

	var nextReview time.Time
	switch reviewCount {
	case 0:
		nextReview = time.Now().AddDate(0, 0, 7)
	case 1:
		nextReview = time.Now().AddDate(0, 0, 15)
	default:
		nextReview = time.Now().AddDate(0, 1, 0)
	}

	_, err := database.DB.Exec(context.Background(),
		`UPDATE daily_content_delivery SET viewed_at=NOW(), next_review_at=$1, review_count=review_count+1
		 WHERE content_id=$2 AND user_id=$3`,
		nextReview, contentID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "marked as viewed"})
}

func UserGetDailyStreak(c *gin.Context) {
	userIDRaw, _ := c.Get("user_id")
	userID := userIDRaw.(int)

	var totalVocab, totalMath, totalScience, totalNews int
	var totalDays int

	_ = database.DB.QueryRow(context.Background(),
		`SELECT COUNT(DISTINCT dcd.content_id) FROM daily_content_delivery dcd
		 JOIN daily_content dc ON dcd.content_id = dc.id
		 WHERE dcd.user_id=$1 AND dc.content_type='vocabulary'`, userID).Scan(&totalVocab)
	_ = database.DB.QueryRow(context.Background(),
		`SELECT COUNT(DISTINCT dcd.content_id) FROM daily_content_delivery dcd
		 JOIN daily_content dc ON dcd.content_id = dc.id
		 WHERE dcd.user_id=$1 AND dc.content_type='math'`, userID).Scan(&totalMath)
	_ = database.DB.QueryRow(context.Background(),
		`SELECT COUNT(DISTINCT dcd.content_id) FROM daily_content_delivery dcd
		 JOIN daily_content dc ON dcd.content_id = dc.id
		 WHERE dcd.user_id=$1 AND dc.content_type='science'`, userID).Scan(&totalScience)
	_ = database.DB.QueryRow(context.Background(),
		`SELECT COUNT(DISTINCT dcd.content_id) FROM daily_content_delivery dcd
		 JOIN daily_content dc ON dcd.content_id = dc.id
		 WHERE dcd.user_id=$1 AND dc.content_type='news'`, userID).Scan(&totalNews)

	_ = database.DB.QueryRow(context.Background(),
		`SELECT COUNT(DISTINCT DATE(delivered_at)) FROM daily_content_delivery WHERE user_id=$1`, userID).Scan(&totalDays)

	c.JSON(http.StatusOK, gin.H{
		"total_vocab":   totalVocab,
		"total_math":    totalMath,
		"total_science": totalScience,
		"total_news":    totalNews,
		"total_days":    totalDays,
	})
}
