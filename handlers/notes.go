package handlers

import (
	"context"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"edunova-server/database"
	"edunova-server/models"
)

func AdminGetNotes(c *gin.Context) {
	classLevel := c.Query("class_level")
	subject := c.Query("subject")

	query := `SELECT id, class_level, subject, title, content, tags, is_published, created_by, created_at, updated_at
		 FROM notes WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if classLevel != "" {
		query += " AND class_level = $" + strconv.Itoa(argIdx)
		args = append(args, classLevel)
		argIdx++
	}
	if subject != "" {
		query += " AND subject = $" + strconv.Itoa(argIdx)
		args = append(args, subject)
		argIdx++
	}
	query += " ORDER BY created_at DESC"

	rows, err := database.DB.Query(context.Background(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	var notes []models.Note
	for rows.Next() {
		var n models.Note
		if err := rows.Scan(&n.ID, &n.ClassLevel, &n.Subject, &n.Title, &n.Content, &n.Tags,
			&n.IsPublished, &n.CreatedBy, &n.CreatedAt, &n.UpdatedAt); err != nil {
			continue
		}
		notes = append(notes, n)
	}
	if notes == nil {
		notes = []models.Note{}
	}
	c.JSON(http.StatusOK, notes)
}

func AdminCreateNote(c *gin.Context) {
	var req models.CreateNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	adminEmail := c.GetString("admin_email")
	var adminID int
	_ = database.DB.QueryRow(context.Background(),
		`SELECT id FROM admin_users WHERE email = $1`, adminEmail).Scan(&adminID)

	isPublished := true
	if req.IsPublished != nil {
		isPublished = *req.IsPublished
	}
	if req.Tags == nil {
		req.Tags = []string{}
	}

	var id int
	err := database.DB.QueryRow(context.Background(),
		`INSERT INTO notes (class_level, subject, title, content, tags, is_published, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		req.ClassLevel, req.Subject, req.Title, req.Content, req.Tags, isPublished, adminID).Scan(&id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create note"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": id, "message": "note created"})
}

func AdminUpdateNote(c *gin.Context) {
	id := c.Param("id")
	var req models.CreateNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	isPublished := true
	if req.IsPublished != nil {
		isPublished = *req.IsPublished
	}
	if req.Tags == nil {
		req.Tags = []string{}
	}

	_, err := database.DB.Exec(context.Background(),
		`UPDATE notes SET class_level=$1, subject=$2, title=$3, content=$4, tags=$5, is_published=$6, updated_at=NOW()
		 WHERE id=$7`,
		req.ClassLevel, req.Subject, req.Title, req.Content, req.Tags, isPublished, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update note"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "note updated"})
}

func AdminDeleteNote(c *gin.Context) {
	id := c.Param("id")
	_, err := database.DB.Exec(context.Background(), `DELETE FROM notes WHERE id=$1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to delete note"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "note deleted"})
}

func UserGetNotes(c *gin.Context) {
	classLevel := c.Query("class_level")
	subject := c.Query("subject")

	// Auto-filter by user's class if no class_level param provided
	if classLevel == "" {
		userIDRaw, exists := c.Get("user_id")
		if exists {
			userID := userIDRaw.(int)
			_ = database.DB.QueryRow(context.Background(),
				`SELECT COALESCE(student_class, '') FROM users WHERE id = $1`, userID).Scan(&classLevel)
		}
	}

	query := `SELECT id, class_level, subject, title, content, tags, is_published, created_at
		 FROM notes WHERE is_published = TRUE`
	args := []interface{}{}
	argIdx := 1

	if classLevel != "" {
		query += " AND (class_level = $" + strconv.Itoa(argIdx) + " OR class_level = '')"
		args = append(args, classLevel)
		argIdx++
	}
	// If classLevel is empty (user hasn't set class), show all notes
	if subject != "" {
		query += " AND subject = $" + strconv.Itoa(argIdx)
		args = append(args, subject)
		argIdx++
	}
	query += " ORDER BY created_at DESC"

	rows, err := database.DB.Query(context.Background(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	type PublicNote struct {
		ID          int      `json:"id"`
		ClassLevel  string   `json:"class_level"`
		Subject     string   `json:"subject"`
		Title       string   `json:"title"`
		Content     string   `json:"content"`
		Chapter     string   `json:"chapter"`
		Type        string   `json:"type"`
		Language    string   `json:"language"`
		Tags        []string `json:"tags"`
		IsPublished bool     `json:"is_published"`
		UsageCount  int      `json:"usage_count"`
		CreatedAt   string   `json:"created_at"`
	}

	var notes []PublicNote
	for rows.Next() {
		var n PublicNote
		if err := rows.Scan(&n.ID, &n.ClassLevel, &n.Subject, &n.Title, &n.Content, &n.Tags,
			&n.IsPublished, &n.CreatedAt); err != nil {
			log.Printf("UserGetNotes scan error: %v", err)
			continue
		}
		// Fetch extra fields not in the main query
		_ = database.DB.QueryRow(context.Background(),
			`SELECT COALESCE(chapter,''), COALESCE(type,'notes'), COALESCE(language,'bn'), COALESCE(usage_count,0) FROM notes WHERE id=$1`, n.ID,
		).Scan(&n.Chapter, &n.Type, &n.Language, &n.UsageCount)
		notes = append(notes, n)
	}
	if notes == nil {
		notes = []PublicNote{}
	}
	c.JSON(http.StatusOK, notes)
}

func UserGetNoteByID(c *gin.Context) {
	id := c.Param("id")
	var n models.Note
	err := database.DB.QueryRow(context.Background(),
		`SELECT id, class_level, subject, title, content, tags, is_published, created_at
		 FROM notes WHERE id = $1 AND is_published = TRUE`, id,
	).Scan(&n.ID, &n.ClassLevel, &n.Subject, &n.Title, &n.Content, &n.Tags, &n.IsPublished, &n.CreatedAt)
	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "note not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	c.JSON(http.StatusOK, n)
}
