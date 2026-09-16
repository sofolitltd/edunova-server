package handlers

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"

	"edunova-server/database"
	"edunova-server/models"
)

// ==================== Class Handlers ====================

func AdminGetClasses(c *gin.Context) {
	rows, err := database.DB.Query(context.Background(),
		`SELECT id, name, name_bn, order_index, COALESCE(code, ''), created_at FROM classes ORDER BY order_index, name`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch classes"})
		return
	}
	defer rows.Close()

	var classes []models.Class
	for rows.Next() {
		var cl models.Class
		if err := rows.Scan(&cl.ID, &cl.Name, &cl.NameBn, &cl.OrderIndex, &cl.Code, &cl.CreatedAt); err != nil {
			continue
		}
		classes = append(classes, cl)
	}
	if classes == nil {
		classes = []models.Class{}
	}
	c.JSON(http.StatusOK, classes)
}

func AdminCreateClass(c *gin.Context) {
	var req models.CreateClassRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var cl models.Class
	err := database.DB.QueryRow(context.Background(),
		`INSERT INTO classes (name, name_bn, order_index, code) VALUES ($1, $2, $3, $4)
		 RETURNING id, name, name_bn, order_index, COALESCE(code, ''), created_at`,
		req.Name, req.NameBn, req.OrderIndex, req.Code,
	).Scan(&cl.ID, &cl.Name, &cl.NameBn, &cl.OrderIndex, &cl.Code, &cl.CreatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create class"})
		return
	}
	c.JSON(http.StatusCreated, cl)
}

func AdminUpdateClass(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req models.CreateClassRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var cl models.Class
	err := database.DB.QueryRow(context.Background(),
		`UPDATE classes SET name=$1, name_bn=$2, order_index=$3, code=$4 WHERE id=$5
		 RETURNING id, name, name_bn, order_index, COALESCE(code, ''), created_at`,
		req.Name, req.NameBn, req.OrderIndex, req.Code, id,
	).Scan(&cl.ID, &cl.Name, &cl.NameBn, &cl.OrderIndex, &cl.Code, &cl.CreatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update class"})
		return
	}
	c.JSON(http.StatusOK, cl)
}

func AdminDeleteClass(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, err := database.DB.Exec(context.Background(), `DELETE FROM classes WHERE id=$1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to delete class"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "class deleted"})
}

// ==================== Subject Handlers ====================

func AdminGetSubjects(c *gin.Context) {
	rows, err := database.DB.Query(context.Background(),
		`SELECT id, name, name_bn, created_at FROM subjects ORDER BY name`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch subjects"})
		return
	}
	defer rows.Close()

	var subjects []models.Subject
	for rows.Next() {
		var s models.Subject
		if err := rows.Scan(&s.ID, &s.Name, &s.NameBn, &s.CreatedAt); err != nil {
			continue
		}
		subjects = append(subjects, s)
	}
	if subjects == nil {
		subjects = []models.Subject{}
	}
	c.JSON(http.StatusOK, subjects)
}

func AdminCreateSubject(c *gin.Context) {
	var req models.CreateSubjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var s models.Subject
	err := database.DB.QueryRow(context.Background(),
		`INSERT INTO subjects (name, name_bn) VALUES ($1, $2)
		 RETURNING id, name, name_bn, created_at`,
		req.Name, req.NameBn,
	).Scan(&s.ID, &s.Name, &s.NameBn, &s.CreatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create subject"})
		return
	}
	c.JSON(http.StatusCreated, s)
}

func AdminUpdateSubject(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req models.CreateSubjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var s models.Subject
	err := database.DB.QueryRow(context.Background(),
		`UPDATE subjects SET name=$1, name_bn=$2 WHERE id=$3
		 RETURNING id, name, name_bn, created_at`,
		req.Name, req.NameBn, id,
	).Scan(&s.ID, &s.Name, &s.NameBn, &s.CreatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update subject"})
		return
	}
	c.JSON(http.StatusOK, s)
}

func AdminDeleteSubject(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, err := database.DB.Exec(context.Background(), `DELETE FROM subjects WHERE id=$1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to delete subject"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "subject deleted"})
}

// ==================== Book Handlers ====================

func AdminGetBooks(c *gin.Context) {
	subjectID := c.Query("subject_id")
	classID := c.Query("class_id")

	query := `SELECT b.id, b.subject_id, s.name as subject_name, COALESCE(c.name,'') as class_name,
	   b.name, b.name_bn, b.publisher, b.created_at
	  FROM books b
	  LEFT JOIN subjects s ON b.subject_id = s.id
	  LEFT JOIN classes c ON b.class_id = c.id
	  WHERE 1=1`
	args := []interface{}{}

	if subjectID != "" {
		query += fmt.Sprintf(" AND b.subject_id = $%d", len(args)+1)
		args = append(args, subjectID)
	}
	if classID != "" {
		query += fmt.Sprintf(" AND b.class_id = $%d", len(args)+1)
		args = append(args, classID)
	}
	query += " ORDER BY s.name, b.name"

	rows, err := database.DB.Query(context.Background(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch books"})
		return
	}
	defer rows.Close()

	var books []models.Book
	for rows.Next() {
		var b models.Book
		if err := rows.Scan(&b.ID, &b.SubjectID, &b.SubjectName, &b.ClassName,
			&b.Name, &b.NameBn, &b.Publisher, &b.CreatedAt); err != nil {
			continue
		}
		books = append(books, b)
	}
	if books == nil {
		books = []models.Book{}
	}
	c.JSON(http.StatusOK, books)
}

func AdminCreateBook(c *gin.Context) {
	var req models.CreateBookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var b models.Book
	err := database.DB.QueryRow(context.Background(),
		`INSERT INTO books (subject_id, class_id, name, name_bn, publisher) VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, subject_id, name, name_bn, publisher, created_at`,
		req.SubjectID, req.ClassID, req.Name, req.NameBn, req.Publisher,
	).Scan(&b.ID, &b.SubjectID, &b.Name, &b.NameBn, &b.Publisher, &b.CreatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create book"})
		return
	}
	c.JSON(http.StatusCreated, b)
}

func AdminUpdateBook(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req models.CreateBookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var b models.Book
	err := database.DB.QueryRow(context.Background(),
		`UPDATE books SET subject_id=$1, class_id=$2, name=$3, name_bn=$4, publisher=$5 WHERE id=$6
		 RETURNING id, subject_id, name, name_bn, publisher, created_at`,
		req.SubjectID, req.ClassID, req.Name, req.NameBn, req.Publisher, id,
	).Scan(&b.ID, &b.SubjectID, &b.Name, &b.NameBn, &b.Publisher, &b.CreatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update book"})
		return
	}
	c.JSON(http.StatusOK, b)
}

func AdminDeleteBook(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, err := database.DB.Exec(context.Background(), `DELETE FROM books WHERE id=$1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to delete book"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "book deleted"})
}

// ==================== Chapter Handlers ====================

func AdminGetChapters(c *gin.Context) {
	bookID := c.Query("book_id")
	if bookID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "book_id is required"})
		return
	}

	rows, err := database.DB.Query(context.Background(),
		`SELECT ch.id, ch.book_id, b.name as book_name, ch.name, ch.name_bn, ch.order_index, ch.created_at
		 FROM chapters ch
		 LEFT JOIN books b ON ch.book_id = b.id
		 WHERE ch.book_id = $1
		 ORDER BY ch.order_index, ch.name`, bookID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch chapters"})
		return
	}
	defer rows.Close()

	var chapters []models.Chapter
	for rows.Next() {
		var ch models.Chapter
		if err := rows.Scan(&ch.ID, &ch.BookID, &ch.BookName, &ch.Name, &ch.NameBn, &ch.OrderIndex, &ch.CreatedAt); err != nil {
			continue
		}
		chapters = append(chapters, ch)
	}
	if chapters == nil {
		chapters = []models.Chapter{}
	}
	c.JSON(http.StatusOK, chapters)
}

func AdminCreateChapter(c *gin.Context) {
	var req models.CreateChapterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var ch models.Chapter
	err := database.DB.QueryRow(context.Background(),
		`INSERT INTO chapters (book_id, name, name_bn, order_index) VALUES ($1, $2, $3, $4)
		 RETURNING id, book_id, name, name_bn, order_index, created_at`,
		req.BookID, req.Name, req.NameBn, req.OrderIndex,
	).Scan(&ch.ID, &ch.BookID, &ch.Name, &ch.NameBn, &ch.OrderIndex, &ch.CreatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create chapter"})
		return
	}
	c.JSON(http.StatusCreated, ch)
}

func AdminUpdateChapter(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req models.CreateChapterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var ch models.Chapter
	err := database.DB.QueryRow(context.Background(),
		`UPDATE chapters SET book_id=$1, name=$2, name_bn=$3, order_index=$4 WHERE id=$5
		 RETURNING id, book_id, name, name_bn, order_index, created_at`,
		req.BookID, req.Name, req.NameBn, req.OrderIndex, id,
	).Scan(&ch.ID, &ch.BookID, &ch.Name, &ch.NameBn, &ch.OrderIndex, &ch.CreatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update chapter"})
		return
	}
	c.JSON(http.StatusOK, ch)
}

func AdminDeleteChapter(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, err := database.DB.Exec(context.Background(), `DELETE FROM chapters WHERE id=$1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to delete chapter"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "chapter deleted"})
}

// ==================== Topic Handlers ====================

func AdminGetTopics(c *gin.Context) {
	chapterID := c.Query("chapter_id")
	if chapterID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "chapter_id is required"})
		return
	}

	rows, err := database.DB.Query(context.Background(),
		`SELECT t.id, t.chapter_id, ch.name as chapter_name, t.name, t.name_bn, t.order_index, t.created_at
		 FROM topics t
		 LEFT JOIN chapters ch ON t.chapter_id = ch.id
		 WHERE t.chapter_id = $1
		 ORDER BY t.order_index, t.name`, chapterID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch topics"})
		return
	}
	defer rows.Close()

	var topics []models.Topic
	for rows.Next() {
		var t models.Topic
		if err := rows.Scan(&t.ID, &t.ChapterID, &t.ChapterName, &t.Name, &t.NameBn, &t.OrderIndex, &t.CreatedAt); err != nil {
			continue
		}
		topics = append(topics, t)
	}
	if topics == nil {
		topics = []models.Topic{}
	}
	c.JSON(http.StatusOK, topics)
}

func AdminCreateTopic(c *gin.Context) {
	var req models.CreateTopicRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var t models.Topic
	err := database.DB.QueryRow(context.Background(),
		`INSERT INTO topics (chapter_id, name, name_bn, order_index) VALUES ($1, $2, $3, $4)
		 RETURNING id, chapter_id, name, name_bn, order_index, created_at`,
		req.ChapterID, req.Name, req.NameBn, req.OrderIndex,
	).Scan(&t.ID, &t.ChapterID, &t.Name, &t.NameBn, &t.OrderIndex, &t.CreatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create topic"})
		return
	}
	c.JSON(http.StatusCreated, t)
}

func AdminUpdateTopic(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req models.CreateTopicRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var t models.Topic
	err := database.DB.QueryRow(context.Background(),
		`UPDATE topics SET chapter_id=$1, name=$2, name_bn=$3, order_index=$4 WHERE id=$5
		 RETURNING id, chapter_id, name, name_bn, order_index, created_at`,
		req.ChapterID, req.Name, req.NameBn, req.OrderIndex, id,
	).Scan(&t.ID, &t.ChapterID, &t.Name, &t.NameBn, &t.OrderIndex, &t.CreatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update topic"})
		return
	}
	c.JSON(http.StatusOK, t)
}

func AdminDeleteTopic(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, err := database.DB.Exec(context.Background(), `DELETE FROM topics WHERE id=$1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to delete topic"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "topic deleted"})
}

// ==================== Question Handlers ====================

func scanQuestion(row interface{ Scan(...interface{}) error }) (models.Question, error) {
	var q models.Question
	var options, tags *string
	err := row.Scan(
		&q.ID, &q.ClassID, &q.ClassName, &q.SubjectID, &q.SubjectName,
		&q.BookID, &q.BookName, &q.ChapterID, &q.ChapterName,
		&q.TopicID, &q.TopicName, &q.QuestionType, &q.QuestionText,
		&options, &q.Answer, &q.Explanation, &q.Marks, &q.Difficulty,
		&tags, &q.Source, &q.SourcePage, &q.Language, &q.Status,
		&q.CreatedBy, &q.ReviewedBy, &q.CreatedAt, &q.UpdatedAt,
		&q.Version, &q.UsageCount, &q.LastUsedAt,
	)
	if options != nil {
		q.Options = options
	}
	if tags != nil {
		s := *tags
		s = strings.Trim(s, "{}")
		if s != "" {
			q.Tags = strings.Split(s, ",")
		} else {
			q.Tags = []string{}
		}
	} else {
		q.Tags = []string{}
	}
	return q, err
}

const questionSelect = `SELECT q.id,
	CASE WHEN q.class_id IS NOT NULL THEN q.class_id END,
	COALESCE(c.name,''), CASE WHEN q.subject_id IS NOT NULL THEN q.subject_id END,
	COALESCE(s.name,''), CASE WHEN q.book_id IS NOT NULL THEN q.book_id END,
	COALESCE(b.name,''), CASE WHEN q.chapter_id IS NOT NULL THEN q.chapter_id END,
	COALESCE(ch.name,''), CASE WHEN q.topic_id IS NOT NULL THEN q.topic_id END,
	COALESCE(t.name,''),
	q.question_type, q.question_text, q.options::text, q.answer, q.explanation,
	q.marks, q.difficulty, ARRAY_TO_STRING(q.tags,','), q.source, q.source_page,
	q.language, q.status, q.created_by, q.reviewed_by,
	q.created_at, q.updated_at, q.version, q.usage_count, q.last_used_at
	FROM questions q
	LEFT JOIN classes c ON q.class_id = c.id
	LEFT JOIN subjects s ON q.subject_id = s.id
	LEFT JOIN books b ON q.book_id = b.id
	LEFT JOIN chapters ch ON q.chapter_id = ch.id
	LEFT JOIN topics t ON q.topic_id = t.id`

func AdminGetQuestions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	where := []string{"1=1"}
	args := []interface{}{}
	argIdx := 1

	if v := c.Query("class_id"); v != "" {
		where = append(where, fmt.Sprintf("q.class_id = $%d", argIdx))
		args = append(args, v)
		argIdx++
	}
	if v := c.Query("subject_id"); v != "" {
		where = append(where, fmt.Sprintf("q.subject_id = $%d", argIdx))
		args = append(args, v)
		argIdx++
	}
	if v := c.Query("book_id"); v != "" {
		where = append(where, fmt.Sprintf("q.book_id = $%d", argIdx))
		args = append(args, v)
		argIdx++
	}
	if v := c.Query("chapter_id"); v != "" {
		where = append(where, fmt.Sprintf("q.chapter_id = $%d", argIdx))
		args = append(args, v)
		argIdx++
	}
	if v := c.Query("topic_id"); v != "" {
		where = append(where, fmt.Sprintf("q.topic_id = $%d", argIdx))
		args = append(args, v)
		argIdx++
	}
	if v := c.Query("question_type"); v != "" {
		where = append(where, fmt.Sprintf("q.question_type = $%d", argIdx))
		args = append(args, v)
		argIdx++
	}
	if v := c.Query("difficulty"); v != "" {
		where = append(where, fmt.Sprintf("q.difficulty = $%d", argIdx))
		args = append(args, v)
		argIdx++
	}
	if v := c.Query("status"); v != "" {
		where = append(where, fmt.Sprintf("q.status = $%d", argIdx))
		args = append(args, v)
		argIdx++
	}
	if v := c.Query("search"); v != "" {
		where = append(where, fmt.Sprintf("q.question_text ILIKE $%d", argIdx))
		args = append(args, "%"+v+"%")
		argIdx++
	}

	whereStr := strings.Join(where, " AND ")

	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM questions q %s", "")
	database.DB.QueryRow(context.Background(), countQuery).Scan(&total)

	query := fmt.Sprintf("%s WHERE %s ORDER BY q.created_at DESC LIMIT $%d OFFSET $%d",
		questionSelect, whereStr, argIdx, argIdx+1)
	args = append(args, perPage, offset)

	rows, err := database.DB.Query(context.Background(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch questions"})
		return
	}
	defer rows.Close()

	var questions []models.Question
	for rows.Next() {
		q, err := scanQuestion(rows)
		if err != nil {
			continue
		}
		questions = append(questions, q)
	}
	if questions == nil {
		questions = []models.Question{}
	}

	totalPages := total / perPage
	if total%perPage > 0 {
		totalPages++
	}

	c.JSON(http.StatusOK, models.PaginatedQuestions{
		Questions:  questions,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	})
}

func AdminCreateQuestion(c *gin.Context) {
	var req models.CreateQuestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	if req.Marks == 0 {
		req.Marks = 1
	}
	if req.Difficulty == "" {
		req.Difficulty = "medium"
	}
	if req.Language == "" {
		req.Language = "bn"
	}
	if req.QuestionType == "" {
		req.QuestionType = "mcq"
	}

	var q models.Question
	err := database.DB.QueryRow(context.Background(),
		`INSERT INTO questions (class_id, subject_id, book_id, chapter_id, topic_id,
		 question_type, question_text, options, answer, explanation,
		 marks, difficulty, tags, source, source_page, language, status)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		 RETURNING id, class_id, subject_id, book_id, chapter_id, topic_id,
		 question_type, question_text, options::text, answer, explanation,
		 marks, difficulty, ARRAY_TO_STRING(tags,','), source, source_page,
		 language, status, created_by, reviewed_by, created_at, updated_at,
		 version, usage_count, last_used_at`,
		req.ClassID, req.SubjectID, req.BookID, req.ChapterID, req.TopicID,
		req.QuestionType, req.QuestionText, req.Options, req.Answer, req.Explanation,
		req.Marks, req.Difficulty, req.Tags, req.Source, req.SourcePage, req.Language, "draft",
	).Scan(&q.ID, &q.ClassID, &q.SubjectID, &q.BookID, &q.ChapterID, &q.TopicID,
		&q.QuestionType, &q.QuestionText, &q.Options, &q.Answer, &q.Explanation,
		&q.Marks, &q.Difficulty, &q.Tags, &q.Source, &q.SourcePage, &q.Language,
		&q.Status, &q.CreatedBy, &q.ReviewedBy, &q.CreatedAt, &q.UpdatedAt,
		&q.Version, &q.UsageCount, &q.LastUsedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create question"})
		return
	}
	c.JSON(http.StatusCreated, q)
}

func AdminGetQuestion(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	row := database.DB.QueryRow(context.Background(), questionSelect+` WHERE q.id=$1`, id)
	q, err := scanQuestion(row)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "question not found"})
		return
	}
	c.JSON(http.StatusOK, q)
}

func AdminUpdateQuestion(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req models.CreateQuestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var q models.Question
	err := database.DB.QueryRow(context.Background(),
		`UPDATE questions SET class_id=$1, subject_id=$2, book_id=$3, chapter_id=$4, topic_id=$5,
		 question_type=$6, question_text=$7, options=$8, answer=$9, explanation=$10,
		 marks=$11, difficulty=$12, tags=$13, source=$14, source_page=$15, language=$16,
		 updated_at=NOW(), version=version+1
		 WHERE id=$17
		 RETURNING id, class_id, subject_id, book_id, chapter_id, topic_id,
		 question_type, question_text, options::text, answer, explanation,
		 marks, difficulty, ARRAY_TO_STRING(tags,','), source, source_page,
		 language, status, created_by, reviewed_by, created_at, updated_at,
		 version, usage_count, last_used_at`,
		req.ClassID, req.SubjectID, req.BookID, req.ChapterID, req.TopicID,
		req.QuestionType, req.QuestionText, req.Options, req.Answer, req.Explanation,
		req.Marks, req.Difficulty, req.Tags, req.Source, req.SourcePage, req.Language, id,
	).Scan(&q.ID, &q.ClassID, &q.SubjectID, &q.BookID, &q.ChapterID, &q.TopicID,
		&q.QuestionType, &q.QuestionText, &q.Options, &q.Answer, &q.Explanation,
		&q.Marks, &q.Difficulty, &q.Tags, &q.Source, &q.SourcePage, &q.Language,
		&q.Status, &q.CreatedBy, &q.ReviewedBy, &q.CreatedAt, &q.UpdatedAt,
		&q.Version, &q.UsageCount, &q.LastUsedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update question"})
		return
	}
	c.JSON(http.StatusOK, q)
}

func AdminDeleteQuestion(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, err := database.DB.Exec(context.Background(), `DELETE FROM questions WHERE id=$1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to delete question"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "question deleted"})
}

func AdminUpdateQuestionStatus(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req models.UpdateQuestionStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	_, err := database.DB.Exec(context.Background(),
		`UPDATE questions SET status=$1, updated_at=NOW() WHERE id=$2`, req.Status, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update status"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "status updated"})
}

// ==================== Duplicate Detection ====================

func normalizeText(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			return ' '
		}
		return r
	}, s)
	// collapse multiple spaces
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

func checkDuplicate(ctx context.Context, questionText string) (*models.DuplicateCheckResult, error) {
	normalized := normalizeText(questionText)

	// Exact match
	var exactID int
	err := database.DB.QueryRow(ctx,
		`SELECT id FROM questions WHERE question_text = $1 LIMIT 1`, questionText,
	).Scan(&exactID)
	if err == nil {
		return &models.DuplicateCheckResult{
			IsDuplicate: true,
			Reason:      "exact match",
			SimilarID:   &exactID,
		}, nil
	}

	// Normalized match
	var normID int
	err = database.DB.QueryRow(ctx,
		`SELECT id FROM questions WHERE LOWER(REPLACE(REPLACE(REPLACE(REPLACE(question_text,
		 E'\t',' '),E'\n',' '),E'\r',''),'  ',' ')) = $1 LIMIT 1`, normalized,
	).Scan(&normID)
	if err == nil {
		return &models.DuplicateCheckResult{
			IsDuplicate: true,
			Reason:      "normalized text match",
			SimilarID:   &normID,
		}, nil
	}

	return &models.DuplicateCheckResult{IsDuplicate: false}, nil
}

func AdminCheckDuplicate(c *gin.Context) {
	var req struct {
		QuestionText string `json:"question_text" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	result, err := checkDuplicate(c.Request.Context(), req.QuestionText)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to check duplicate"})
		return
	}
	c.JSON(http.StatusOK, result)
}

// ==================== Bulk CSV Import ====================

func processCSVRecords(records [][]string) models.BulkUploadResult {
	header := normalizeCSVHeader(records[0])
	requiredCols := []string{"class", "subject", "book", "chapter", "question_type", "question", "answer"}
	for _, col := range requiredCols {
		if _, ok := header[col]; !ok {
			return models.BulkUploadResult{
				Errors:    1,
				ErrorRows: []models.ImportErrorRow{{Row: 1, Errors: []string{fmt.Sprintf("missing required column: %s", col)}}},
			}
		}
	}

	var imported, duplicates, errors int
	var errorRows []models.ImportErrorRow
	ctx := context.Background()

	for i, row := range records[1:] {
		rowNum := i + 2
		var rowErrors []string

		questionText := getFieldByHeader(row, header, "question")
		answer := getFieldByHeader(row, header, "answer")
		className := getFieldByHeader(row, header, "class")
		subjectName := getFieldByHeader(row, header, "subject")
		bookName := getFieldByHeader(row, header, "book")
		chapterName := getFieldByHeader(row, header, "chapter")
		questionType := getFieldByHeader(row, header, "question_type")
		optionA := getFieldByHeader(row, header, "option_a")
		optionB := getFieldByHeader(row, header, "option_b")
		optionC := getFieldByHeader(row, header, "option_c")
		optionD := getFieldByHeader(row, header, "option_d")
		explanation := getFieldByHeader(row, header, "explanation")
		difficulty := getFieldByHeader(row, header, "difficulty")
		tagsStr := getFieldByHeader(row, header, "tags")

		if questionText == "" {
			rowErrors = append(rowErrors, "question is required")
		}
		if answer == "" {
			rowErrors = append(rowErrors, "answer is required")
		}
		if questionType == "" {
			questionType = "mcq"
		}
		if difficulty == "" {
			difficulty = "medium"
		}

		dup, _ := checkDuplicate(ctx, questionText)
		if dup != nil && dup.IsDuplicate {
			duplicates++
			continue
		}

		if len(rowErrors) > 0 {
			errors++
			errorRows = append(errorRows, models.ImportErrorRow{Row: rowNum, Errors: rowErrors})
			continue
		}

		var classID, subjectID, bookID, chapterID *int
		if className != "" {
			var id int
			if err := database.DB.QueryRow(ctx, `SELECT id FROM classes WHERE name ILIKE $1`, className).Scan(&id); err == nil {
				classID = &id
			}
		}
		if subjectName != "" {
			var id int
			if err := database.DB.QueryRow(ctx, `SELECT id FROM subjects WHERE name ILIKE $1`, subjectName).Scan(&id); err == nil {
				subjectID = &id
			}
		}
		if bookName != "" && subjectID != nil {
			var id int
			if err := database.DB.QueryRow(ctx, `SELECT id FROM books WHERE name ILIKE $1 AND subject_id=$2`, bookName, *subjectID).Scan(&id); err == nil {
				bookID = &id
			}
		}
		if chapterName != "" && bookID != nil {
			var id int
			if err := database.DB.QueryRow(ctx, `SELECT id FROM chapters WHERE name ILIKE $1 AND book_id=$2`, chapterName, *bookID).Scan(&id); err == nil {
				chapterID = &id
			}
		}

		var optionsJSON *string
		if questionType == "mcq" && (optionA != "" || optionB != "" || optionC != "" || optionD != "") {
			opts := []map[string]string{}
			if optionA != "" {
				opts = append(opts, map[string]string{"label": "A", "text": optionA})
			}
			if optionB != "" {
				opts = append(opts, map[string]string{"label": "B", "text": optionB})
			}
			if optionC != "" {
				opts = append(opts, map[string]string{"label": "C", "text": optionC})
			}
			if optionD != "" {
				opts = append(opts, map[string]string{"label": "D", "text": optionD})
			}
			b, _ := json.Marshal(opts)
			s := string(b)
			optionsJSON = &s
		}

		marks := 1
		if v := getFieldByHeader(row, header, "marks"); v != "" {
			if m, err := strconv.Atoi(v); err == nil {
				marks = m
			}
		}

		var tags []string
		if tagsStr != "" {
			tags = strings.Split(tagsStr, ",")
			for j := range tags {
				tags[j] = strings.TrimSpace(tags[j])
			}
		}

		_, err := database.DB.Exec(ctx,
			`INSERT INTO questions (class_id, subject_id, book_id, chapter_id,
			 question_type, question_text, options, answer, explanation,
			 marks, difficulty, tags, status)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'draft')`,
			classID, subjectID, bookID, chapterID,
			questionType, questionText, optionsJSON, answer, explanation,
			marks, difficulty, tags,
		)
		if err != nil {
			errors++
			errorRows = append(errorRows, models.ImportErrorRow{Row: rowNum, Errors: []string{"database insert failed"}})
			continue
		}
		imported++
	}

	return models.BulkUploadResult{
		TotalRows:  len(records) - 1,
		Imported:   imported,
		Duplicates: duplicates,
		Errors:     errors,
		ErrorRows:  errorRows,
	}
}

func AdminBulkUploadQuestions(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "file is required"})
		return
	}
	defer file.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, file); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "failed to read file"})
		return
	}

	reader := csv.NewReader(bytes.NewReader(buf.Bytes()))
	records, err := reader.ReadAll()
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid CSV format"})
		return
	}

	if len(records) < 2 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "CSV must have header row and at least one data row"})
		return
	}

	result := processCSVRecords(records)

	// Record import job
	errorDetailsJSON, _ := json.Marshal(result.ErrorRows)
	var importID int
	ctx := context.Background()
	err = database.DB.QueryRow(ctx,
		`INSERT INTO question_imports (filename, total_rows, imported, duplicates, errors, error_details, status, completed_at)
		 VALUES ($1, $2, $3, $4, $5, $6, 'completed', NOW())
		 RETURNING id`,
		"upload", result.TotalRows, result.Imported, result.Duplicates, result.Errors, string(errorDetailsJSON),
	).Scan(&importID)
	if err != nil {
		importID = 0
	}

	result.ImportID = importID
	c.JSON(http.StatusOK, result)
}

func AdminFetchGoogleSheet(c *gin.Context) {
	var req struct {
		URL string `json:"url" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "url is required"})
		return
	}

	// Extract sheet ID from various Google Sheets URL formats
	sheetID := ""
	url := req.URL

	if idx := strings.Index(url, "/d/"); idx != -1 {
		rest := url[idx+3:]
		if endIdx := strings.Index(rest, "/"); endIdx != -1 {
			sheetID = rest[:endIdx]
		} else {
			sheetID = rest
		}
	}

	if sheetID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid Google Sheets URL"})
		return
	}

	exportURL := fmt.Sprintf("https://docs.google.com/spreadsheets/d/%s/export?format=csv", sheetID)

	resp, err := http.Get(exportURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "failed to fetch Google Sheet"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: fmt.Sprintf("failed to fetch sheet (status %d) — make sure the sheet is shared with 'Anyone with the link'", resp.StatusCode)})
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to read response"})
		return
	}

	reader := csv.NewReader(bytes.NewReader(body))
	records, err := reader.ReadAll()
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid CSV format in Google Sheet"})
		return
	}

	if len(records) < 2 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Google Sheet must have header row and at least one data row"})
		return
	}

	// Return parsed rows for preview (no DB insert)
	c.JSON(http.StatusOK, gin.H{
		"header": records[0],
		"rows":   records[1:],
		"total":  len(records) - 1,
	})
}

func normalizeCSVHeader(header []string) map[string]int {
	result := map[string]int{}
	for i, h := range header {
		h = strings.ToLower(strings.TrimSpace(h))
		h = strings.ReplaceAll(h, " ", "_")
		result[h] = i
	}
	return result
}

func getFieldByHeader(row []string, header map[string]int, field string) string {
	if idx, ok := header[field]; ok && idx < len(row) {
		return strings.TrimSpace(row[idx])
	}
	return ""
}

// ==================== Question Bank Stats ====================

func AdminGetQuestionBankStats(c *gin.Context) {
	stats := models.QuestionBankStats{
		ByStatus:     map[string]int{},
		ByType:       map[string]int{},
		ByDifficulty: map[string]int{},
	}

	// Total
	database.DB.QueryRow(context.Background(), `SELECT COUNT(*) FROM questions`).Scan(&stats.Total)

	// By status
	rows, _ := database.DB.Query(context.Background(), `SELECT status, COUNT(*) FROM questions GROUP BY status`)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var status string
			var count int
			rows.Scan(&status, &count)
			stats.ByStatus[status] = count
		}
	}

	// By type
	rows, _ = database.DB.Query(context.Background(), `SELECT question_type, COUNT(*) FROM questions GROUP BY question_type`)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var qt string
			var count int
			rows.Scan(&qt, &count)
			stats.ByType[qt] = count
		}
	}

	// By difficulty
	rows, _ = database.DB.Query(context.Background(), `SELECT difficulty, COUNT(*) FROM questions GROUP BY difficulty`)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var d string
			var count int
			rows.Scan(&d, &count)
			stats.ByDifficulty[d] = count
		}
	}

	// By class
	rows, _ = database.DB.Query(context.Background(),
		`SELECT c.id, c.name, COUNT(q.id)
		 FROM classes c LEFT JOIN questions q ON q.class_id = c.id
		 GROUP BY c.id, c.name ORDER BY c.name`)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var cs models.ClassStats
			rows.Scan(&cs.ClassID, &cs.ClassName, &cs.Total)
			cs.ByType = map[string]int{}
			// Get type breakdown for this class
			typeRows, _ := database.DB.Query(context.Background(),
				`SELECT question_type, COUNT(*) FROM questions WHERE class_id=$1 GROUP BY question_type`, cs.ClassID)
			if typeRows != nil {
				for typeRows.Next() {
					var qt string
					var count int
					typeRows.Scan(&qt, &count)
					cs.ByType[qt] = count
				}
				typeRows.Close()
			}
			stats.ByClass = append(stats.ByClass, cs)
		}
	}

	// By chapter
	rows, _ = database.DB.Query(context.Background(),
		`SELECT ch.id, ch.name, COALESCE(c.name,''), COALESCE(s.name,''), COUNT(q.id)
		 FROM chapters ch
		 LEFT JOIN books b ON ch.book_id = b.id
		 LEFT JOIN classes c ON b.class_id = c.id
		 LEFT JOIN subjects s ON b.subject_id = s.id
		 LEFT JOIN questions q ON q.chapter_id = ch.id
		 GROUP BY ch.id, ch.name, c.name, s.name ORDER BY c.name, s.name, ch.name`)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var cs models.ChapterStats
			rows.Scan(&cs.ChapterID, &cs.ChapterName, &cs.ClassName, &cs.SubjectName, &cs.Total)
			cs.ByType = map[string]int{}
			typeRows, _ := database.DB.Query(context.Background(),
				`SELECT question_type, COUNT(*) FROM questions WHERE chapter_id=$1 GROUP BY question_type`, cs.ChapterID)
			if typeRows != nil {
				for typeRows.Next() {
					var qt string
					var count int
					typeRows.Scan(&qt, &count)
					cs.ByType[qt] = count
				}
				typeRows.Close()
			}
			stats.ByChapter = append(stats.ByChapter, cs)
		}
	}

	c.JSON(http.StatusOK, stats)
}

// ==================== Import History ====================

func AdminGetImportHistory(c *gin.Context) {
	rows, err := database.DB.Query(context.Background(),
		`SELECT id, filename, total_rows, imported, duplicates, errors, error_details, status, created_at, completed_at
		 FROM question_imports ORDER BY created_at DESC LIMIT 50`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch import history"})
		return
	}
	defer rows.Close()

	var imports []models.QuestionImport
	for rows.Next() {
		var imp models.QuestionImport
		if err := rows.Scan(&imp.ID, &imp.Filename, &imp.TotalRows, &imp.Imported,
			&imp.Duplicates, &imp.Errors, &imp.ErrorDetails, &imp.Status,
			&imp.CreatedAt, &imp.CompletedAt); err != nil {
			continue
		}
		imports = append(imports, imp)
	}
	if imports == nil {
		imports = []models.QuestionImport{}
	}
	c.JSON(http.StatusOK, imports)
}

// ==================== Bulk Hierarchy Import ====================

func AdminBulkImportHierarchy(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "file is required"})
		return
	}

	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to open file"})
		return
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid CSV format"})
		return
	}

	if len(records) < 2 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "CSV must have a header row and at least one data row"})
		return
	}

	ctx := context.Background()
	var result models.BulkHierarchyResult

	// Helpers
	getOrInsertBn := func(table, name, nameBn string) (int, error) {
		var id int
		err := database.DB.QueryRow(ctx, `SELECT id FROM `+table+` WHERE name = $1`, name).Scan(&id)
		if err == nil {
			return id, nil
		}
		err = database.DB.QueryRow(ctx, `INSERT INTO `+table+` (name, name_bn) VALUES ($1, $2) RETURNING id`, name, nameBn).Scan(&id)
		return id, err
	}

	getOrInsertBook := func(subjectID, classID int, name, nameBn, publisher string) (int, error) {
		var id int
		err := database.DB.QueryRow(ctx, `SELECT id FROM books WHERE name = $1 AND subject_id = $2 AND class_id = $3`, name, subjectID, classID).Scan(&id)
		if err == nil {
			return id, nil
		}
		err = database.DB.QueryRow(ctx, `INSERT INTO books (subject_id, class_id, name, name_bn, publisher) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
			subjectID, classID, name, nameBn, publisher).Scan(&id)
		return id, err
	}

	getOrInsertChapter := func(bookID int, name, nameBn string, orderIndex int) (int, error) {
		var id int
		err := database.DB.QueryRow(ctx, `SELECT id FROM chapters WHERE name = $1 AND book_id = $2`, name, bookID).Scan(&id)
		if err == nil {
			return id, nil
		}
		err = database.DB.QueryRow(ctx, `INSERT INTO chapters (book_id, name, name_bn, order_index) VALUES ($1,$2,$3,$4) RETURNING id`,
			bookID, name, nameBn, orderIndex).Scan(&id)
		return id, err
	}

	getOrInsertTopic := func(chapterID int, name, nameBn string, orderIndex int) (int, error) {
		var id int
		err := database.DB.QueryRow(ctx, `SELECT id FROM topics WHERE name = $1 AND chapter_id = $2`, name, chapterID).Scan(&id)
		if err == nil {
			return id, nil
		}
		err = database.DB.QueryRow(ctx, `INSERT INTO topics (chapter_id, name, name_bn, order_index) VALUES ($1,$2,$3,$4) RETURNING id`,
			chapterID, name, nameBn, orderIndex).Scan(&id)
		return id, err
	}

	for i, row := range records {
		if i == 0 {
			continue // skip header
		}
		if len(row) < 6 {
			result.Errors = append(result.Errors, fmt.Sprintf("Row %d: not enough columns (need at least class, subject, book, chapter, topic, publisher)", i+1))
			continue
		}

		className := strings.TrimSpace(row[0])
		classBn := strings.TrimSpace(row[1])
		subjectName := strings.TrimSpace(row[2])
		subjectBn := strings.TrimSpace(row[3])
		bookName := strings.TrimSpace(row[4])
		bookBn := strings.TrimSpace(row[5])
		publisher := ""
		if len(row) > 6 {
			publisher = strings.TrimSpace(row[6])
		}
		chapterName := ""
		chapterBn := ""
		if len(row) > 7 {
			chapterName = strings.TrimSpace(row[7])
			chapterBn = strings.TrimSpace(row[8])
		}
		topicName := ""
		topicBn := ""
		if len(row) > 9 {
			topicName = strings.TrimSpace(row[9])
			topicBn = strings.TrimSpace(row[10])
		}

		if className == "" || subjectName == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("Row %d: class and subject are required", i+1))
			continue
		}

		classID, err := getOrInsertBn("classes", className, classBn)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Row %d class: %v", i+1, err))
			continue
		}
		subjectID, err := getOrInsertBn("subjects", subjectName, subjectBn)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Row %d subject: %v", i+1, err))
			continue
		}

		if bookName != "" {
			bookID, err := getOrInsertBook(subjectID, classID, bookName, bookBn, publisher)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("Row %d book: %v", i+1, err))
				continue
			}

			if chapterName != "" {
				chapterID, err := getOrInsertChapter(bookID, chapterName, chapterBn, 0)
				if err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("Row %d chapter: %v", i+1, err))
					continue
				}

				if topicName != "" {
					_, err := getOrInsertTopic(chapterID, topicName, topicBn, 0)
					if err != nil {
						result.Errors = append(result.Errors, fmt.Sprintf("Row %d topic: %v", i+1, err))
						continue
					}
				}
			}
		}

		result.Created++
	}

	c.JSON(http.StatusOK, result)
}
