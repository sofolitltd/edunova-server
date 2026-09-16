package handlers

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"edunova-server/database"
	"edunova-server/middleware"
	"edunova-server/models"
)

func AdminLogin(c *gin.Context) {
	var req models.AdminLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var admin models.AdminUser
	err := database.DB.QueryRow(
		context.Background(),
		`SELECT id, email, password_hash, full_name, COALESCE(role, 'admin'), created_at, updated_at
		 FROM admin_users WHERE email = $1`,
		req.Email,
	).Scan(&admin.ID, &admin.Email, &admin.PasswordHash, &admin.FullName, &admin.Role, &admin.CreatedAt, &admin.UpdatedAt)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "invalid email or password"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "invalid email or password"})
		return
	}

	token, err := middleware.GenerateAdminToken(admin.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, models.AdminAuthResponse{Token: token, Admin: admin})
}

func AdminDashboard(c *gin.Context) {
	var stats models.DashboardStats

	_ = database.DB.QueryRow(context.Background(), `SELECT COUNT(*) FROM users`).Scan(&stats.TotalUsers)
	_ = database.DB.QueryRow(context.Background(), `SELECT COUNT(*) FROM users WHERE verified = true`).Scan(&stats.VerifiedUsers)
	_ = database.DB.QueryRow(context.Background(), `SELECT COUNT(*) FROM courses`).Scan(&stats.TotalCourses)
	_ = database.DB.QueryRow(context.Background(), `SELECT COUNT(*) FROM exams`).Scan(&stats.TotalExams)

	c.JSON(http.StatusOK, stats)
}

func AdminGetProfile(c *gin.Context) {
	email := c.GetString("admin_email")

	var admin models.AdminUser
	err := database.DB.QueryRow(context.Background(),
		`SELECT id, email, full_name, COALESCE(role, 'admin'), created_at, updated_at FROM admin_users WHERE email = $1`, email,
	).Scan(&admin.ID, &admin.Email, &admin.FullName, &admin.Role, &admin.CreatedAt, &admin.UpdatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "admin not found"})
		return
	}
	c.JSON(http.StatusOK, admin)
}

func AdminUpdateProfile(c *gin.Context) {
	email := c.GetString("admin_email")

	var req models.UpdateAdminProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	// Check if email is taken by another admin
	var count int
	_ = database.DB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM admin_users WHERE email = $1 AND email != $2`, req.Email, email,
	).Scan(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, models.ErrorResponse{Error: "email already taken"})
		return
	}

	var admin models.AdminUser
	err := database.DB.QueryRow(context.Background(),
		`UPDATE admin_users SET full_name=$1, email=$2, updated_at=NOW()
		 WHERE email=$3
		 RETURNING id, email, full_name, COALESCE(role, 'admin'), created_at, updated_at`,
		req.FullName, req.Email, email,
	).Scan(&admin.ID, &admin.Email, &admin.FullName, &admin.Role, &admin.CreatedAt, &admin.UpdatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update profile"})
		return
	}
	c.JSON(http.StatusOK, admin)
}

func AdminChangePassword(c *gin.Context) {
	email := c.GetString("admin_email")

	var req models.ChangeAdminPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var passwordHash string
	err := database.DB.QueryRow(context.Background(),
		`SELECT password_hash FROM admin_users WHERE email = $1`, email,
	).Scan(&passwordHash)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "admin not found"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "incorrect current password"})
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to hash password"})
		return
	}

	_, err = database.DB.Exec(context.Background(),
		`UPDATE admin_users SET password_hash=$1, updated_at=NOW() WHERE email=$2`,
		string(newHash), email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to change password"})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{Message: "password changed successfully"})
}

func AdminGetUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "10"))
	search := c.Query("search")

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 10
	}
	offset := (page - 1) * perPage

	var total int
	var users []models.User

	if search != "" {
		_ = database.DB.QueryRow(
			context.Background(),
			`SELECT COUNT(*) FROM users WHERE full_name ILIKE $1 OR mobile ILIKE $1`,
			"%"+search+"%",
		).Scan(&total)

		rows, err := database.DB.Query(
			context.Background(),
			`SELECT id, full_name, mobile, verified,
			 COALESCE(father_name,''), COALESCE(father_mobile,''),
			 COALESCE(mother_name,''), COALESCE(mother_mobile,''),
			 COALESCE(student_class,''), created_at, updated_at
			 FROM users WHERE full_name ILIKE $1 OR mobile ILIKE $1
			 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
			"%"+search+"%", perPage, offset,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
			return
		}
		defer rows.Close()

		for rows.Next() {
			var u models.User
			_ = rows.Scan(&u.ID, &u.FullName, &u.Mobile, &u.Verified,
				&u.FatherName, &u.FatherMobile, &u.MotherName, &u.MotherMobile,
				&u.StudentClass, &u.CreatedAt, &u.UpdatedAt)
			users = append(users, u)
		}
	} else {
		_ = database.DB.QueryRow(context.Background(), `SELECT COUNT(*) FROM users`).Scan(&total)

		rows, err := database.DB.Query(
			context.Background(),
			`SELECT id, full_name, mobile, verified,
			 COALESCE(father_name,''), COALESCE(father_mobile,''),
			 COALESCE(mother_name,''), COALESCE(mother_mobile,''),
			 COALESCE(student_class,''), created_at, updated_at
			 FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
			perPage, offset,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
			return
		}
		defer rows.Close()

		for rows.Next() {
			var u models.User
			_ = rows.Scan(&u.ID, &u.FullName, &u.Mobile, &u.Verified,
				&u.FatherName, &u.FatherMobile, &u.MotherName, &u.MotherMobile,
				&u.StudentClass, &u.CreatedAt, &u.UpdatedAt)
			users = append(users, u)
		}
	}

	if users == nil {
		users = []models.User{}
	}

	c.JSON(http.StatusOK, models.PaginatedUsers{
		Users:      users,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: int(math.Ceil(float64(total) / float64(perPage))),
	})
}

func AdminGetUserByID(c *gin.Context) {
	id := c.Param("id")

	var u models.User
	err := database.DB.QueryRow(
		context.Background(),
		`SELECT id, full_name, mobile, verified,
		 COALESCE(father_name,''), COALESCE(father_mobile,''),
		 COALESCE(mother_name,''), COALESCE(mother_mobile,''),
		 COALESCE(notification_mobile,''), COALESCE(gender,''), COALESCE(religion,''),
		 COALESCE(student_class,''), COALESCE(shift,''), COALESCE(school,''),
		 COALESCE(address,''), created_at, updated_at
		 FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.FullName, &u.Mobile, &u.Verified,
		&u.FatherName, &u.FatherMobile, &u.MotherName, &u.MotherMobile,
		&u.NotificationMobile, &u.Gender, &u.Religion,
		&u.StudentClass, &u.Shift, &u.School,
		&u.Address, &u.CreatedAt, &u.UpdatedAt)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "user not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}

	c.JSON(http.StatusOK, u)
}

func AdminToggleVerify(c *gin.Context) {
	id := c.Param("id")

	var verified bool
	err := database.DB.QueryRow(
		context.Background(),
		`UPDATE users SET verified = NOT verified, updated_at = NOW() WHERE id = $1 RETURNING verified`,
		id,
	).Scan(&verified)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "user not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"verified": verified})
}

func AdminDeleteUser(c *gin.Context) {
	id := c.Param("id")

	tag, err := database.DB.Exec(
		context.Background(),
		`DELETE FROM users WHERE id = $1`, id,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "user not found"})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{Message: "user deleted"})
}

const courseColumns = `id, title, title_bn, description, subject, teacher, instructors, class_level, type, schedule, color, gradient,
	price, old_price, duration, badge, students_count, classes_count, exams_count,
	rating, reviews_count, curriculum, features, created_at, updated_at`

func scanCourse(row interface{ Scan(...interface{}) error }) (models.Course, error) {
	var co models.Course
	err := row.Scan(
		&co.ID, &co.Title, &co.TitleBn, &co.Description, &co.Subject, &co.Teacher,
		&co.Instructors, &co.ClassLevel, &co.Type, &co.Schedule, &co.Color, &co.Gradient, &co.Price, &co.OldPrice, &co.Duration,
		&co.Badge, &co.StudentsCount, &co.ClassesCount, &co.ExamsCount, &co.Rating,
		&co.ReviewsCount, &co.Curriculum, &co.Features, &co.CreatedAt, &co.UpdatedAt,
	)
	return co, err
}

func AdminGetCourses(c *gin.Context) {
	rows, err := database.DB.Query(
		context.Background(),
		`SELECT `+courseColumns+` FROM courses ORDER BY created_at DESC`,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	var courses []models.Course
	for rows.Next() {
		co, _ := scanCourse(rows)
		courses = append(courses, co)
	}

	if courses == nil {
		courses = []models.Course{}
	}

	c.JSON(http.StatusOK, courses)
}

func AdminCreateCourse(c *gin.Context) {
	var req models.CreateCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	if req.Color == "" {
		req.Color = "#6366F1"
	}
	if req.Gradient == "" {
		req.Gradient = "from-primary to-primary-dark"
	}
	if req.Type == "" {
		req.Type = "offline"
	}
	if req.Curriculum == nil {
		req.Curriculum = []byte("[]")
	}
	if req.Features == nil {
		req.Features = []byte("[]")
	}

	var course models.Course
	err := database.DB.QueryRow(
		context.Background(),
		`INSERT INTO courses (title, title_bn, description, subject, teacher, instructors, class_level, type, schedule, color, gradient,
			price, old_price, duration, badge, students_count, classes_count, exams_count,
			rating, reviews_count, curriculum, features)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)
		 RETURNING `+courseColumns,
		req.Title, req.TitleBn, req.Description, req.Subject, req.Teacher, req.Instructors, req.ClassLevel, req.Type, req.Schedule,
		req.Color, req.Gradient, req.Price, req.OldPrice, req.Duration, req.Badge,
		req.StudentsCount, req.ClassesCount, req.ExamsCount, req.Rating, req.ReviewsCount,
		req.Curriculum, req.Features,
	).Scan(
		&course.ID, &course.Title, &course.TitleBn, &course.Description, &course.Subject,
		&course.Teacher, &course.Instructors, &course.ClassLevel, &course.Type, &course.Schedule, &course.Color, &course.Gradient, &course.Price,
		&course.OldPrice, &course.Duration, &course.Badge, &course.StudentsCount,
		&course.ClassesCount, &course.ExamsCount, &course.Rating, &course.ReviewsCount,
		&course.Curriculum, &course.Features, &course.CreatedAt, &course.UpdatedAt,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create course"})
		return
	}

	c.JSON(http.StatusCreated, course)
}

func AdminUpdateCourse(c *gin.Context) {
	id := c.Param("id")
	var req models.CreateCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	if req.Color == "" {
		req.Color = "#6366F1"
	}
	if req.Gradient == "" {
		req.Gradient = "from-primary to-primary-dark"
	}
	if req.Curriculum == nil {
		req.Curriculum = []byte("[]")
	}
	if req.Features == nil {
		req.Features = []byte("[]")
	}

	var course models.Course
	err := database.DB.QueryRow(
		context.Background(),
		`UPDATE courses SET title=$1, title_bn=$2, description=$3, subject=$4, teacher=$5,
			instructors=$6, class_level=$7, type=$8, schedule=$9, color=$10, gradient=$11, price=$12, old_price=$13, duration=$14,
			badge=$15, students_count=$16, classes_count=$17, exams_count=$18,
			rating=$19, reviews_count=$20, curriculum=$21, features=$22, updated_at=NOW()
		 WHERE id=$23
		 RETURNING `+courseColumns,
		req.Title, req.TitleBn, req.Description, req.Subject, req.Teacher,
		req.Instructors, req.ClassLevel, req.Type, req.Schedule, req.Color, req.Gradient, req.Price, req.OldPrice, req.Duration,
		req.Badge, req.StudentsCount, req.ClassesCount, req.ExamsCount, req.Rating,
		req.ReviewsCount, req.Curriculum, req.Features, id,
	).Scan(
		&course.ID, &course.Title, &course.TitleBn, &course.Description, &course.Subject,
		&course.Teacher, &course.Instructors, &course.ClassLevel, &course.Type, &course.Schedule, &course.Color, &course.Gradient, &course.Price,
		&course.OldPrice, &course.Duration, &course.Badge, &course.StudentsCount,
		&course.ClassesCount, &course.ExamsCount, &course.Rating, &course.ReviewsCount,
		&course.Curriculum, &course.Features, &course.CreatedAt, &course.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "course not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}

	c.JSON(http.StatusOK, course)
}

func AdminDeleteCourse(c *gin.Context) {
	id := c.Param("id")
	tag, err := database.DB.Exec(context.Background(), `DELETE FROM courses WHERE id = $1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "course not found"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "course deleted"})
}

func AdminGetExams(c *gin.Context) {
	batchID := c.Query("batch_id")

	query := `SELECT e.id, e.title, e.course_id, COALESCE(c.title, ''), COALESCE(e.batch_id, 0), COALESCE(b.name, ''),
		 e.date, e.time, e.duration, e.total_questions, e.created_at, e.updated_at
		 FROM exams e
		 LEFT JOIN courses c ON e.course_id = c.id
		 LEFT JOIN batches b ON e.batch_id = b.id`
	var args []interface{}
	var conditions []string

	if batchID != "" {
		args = append(args, batchID)
		conditions = append(conditions, fmt.Sprintf("e.batch_id = $%d", len(args)))
	}
	if teacherIDs, isTeacher := teacherAssignedBatchIDs(c); isTeacher {
		if len(teacherIDs) == 0 || (batchID != "" && !intSliceContains(teacherIDs, batchID)) {
			c.JSON(http.StatusOK, []models.Exam{})
			return
		}
		if batchID == "" {
			args = append(args, teacherIDs)
			conditions = append(conditions, fmt.Sprintf("e.batch_id = ANY($%d)", len(args)))
		}
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += ` ORDER BY e.created_at DESC`

	rows, err := database.DB.Query(context.Background(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	var exams []models.Exam
	for rows.Next() {
		var ex models.Exam
		_ = rows.Scan(&ex.ID, &ex.Title, &ex.CourseID, &ex.CourseName, &ex.BatchID, &ex.BatchName, &ex.Date, &ex.Time, &ex.Duration, &ex.TotalQuestions, &ex.CreatedAt, &ex.UpdatedAt)
		exams = append(exams, ex)
	}

	if exams == nil {
		exams = []models.Exam{}
	}

	c.JSON(http.StatusOK, exams)
}

func AdminCreateExam(c *gin.Context) {
	var req models.CreateExamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	if !teacherCanUseBatchID(c, req.BatchID) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "not authorized for this batch"})
		return
	}

	var batchID *int
	if req.BatchID != 0 {
		batchID = &req.BatchID
	}

	var exam models.Exam
	err := database.DB.QueryRow(
		context.Background(),
		`INSERT INTO exams (title, course_id, batch_id, date, time, duration, total_questions)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, title, course_id, COALESCE(batch_id, 0), date, time, duration, total_questions, created_at, updated_at`,
		req.Title, req.CourseID, batchID, req.Date, req.Time, req.Duration, len(req.Questions),
	).Scan(&exam.ID, &exam.Title, &exam.CourseID, &exam.BatchID, &exam.Date, &exam.Time, &exam.Duration, &exam.TotalQuestions, &exam.CreatedAt, &exam.UpdatedAt)

	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create exam"})
		return
	}

	for _, q := range req.Questions {
		_, _ = database.DB.Exec(
			context.Background(),
			`INSERT INTO exam_questions (exam_id, question_text, option_a, option_b, option_c, option_d, correct_option)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			exam.ID, q.QuestionText, q.OptionA, q.OptionB, q.OptionC, q.OptionD, q.CorrectOption,
		)
	}

	c.JSON(http.StatusCreated, exam)
}

func AdminUpdateExam(c *gin.Context) {
	id := c.Param("id")
	var req models.CreateExamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	if !teacherOwnsBatchRow(c, "exams", id) || !teacherCanUseBatchID(c, req.BatchID) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "not authorized for this batch"})
		return
	}

	var batchID *int
	if req.BatchID != 0 {
		batchID = &req.BatchID
	}

	var exam models.Exam
	err := database.DB.QueryRow(
		context.Background(),
		`UPDATE exams SET title=$1, course_id=$2, batch_id=$3, date=$4, time=$5, duration=$6, total_questions=$7, updated_at=NOW()
		 WHERE id=$8
		 RETURNING id, title, course_id, COALESCE(batch_id, 0), date, time, duration, total_questions, created_at, updated_at`,
		req.Title, req.CourseID, batchID, req.Date, req.Time, req.Duration, len(req.Questions), id,
	).Scan(&exam.ID, &exam.Title, &exam.CourseID, &exam.BatchID, &exam.Date, &exam.Time, &exam.Duration, &exam.TotalQuestions, &exam.CreatedAt, &exam.UpdatedAt)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "exam not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}

	_, _ = database.DB.Exec(context.Background(), `DELETE FROM exam_questions WHERE exam_id = $1`, exam.ID)
	for _, q := range req.Questions {
		_, _ = database.DB.Exec(
			context.Background(),
			`INSERT INTO exam_questions (exam_id, question_text, option_a, option_b, option_c, option_d, correct_option)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			exam.ID, q.QuestionText, q.OptionA, q.OptionB, q.OptionC, q.OptionD, q.CorrectOption,
		)
	}

	c.JSON(http.StatusOK, exam)
}

func AdminDeleteExam(c *gin.Context) {
	id := c.Param("id")
	if !teacherOwnsBatchRow(c, "exams", id) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "not authorized for this batch"})
		return
	}
	tag, err := database.DB.Exec(context.Background(), `DELETE FROM exams WHERE id = $1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "exam not found"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "exam deleted"})
}

func AdminGetExamQuestions(c *gin.Context) {
	examID := c.Param("id")
	if !teacherOwnsBatchRow(c, "exams", examID) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "not authorized for this batch"})
		return
	}

	rows, err := database.DB.Query(
		context.Background(),
		`SELECT id, exam_id, question_text, option_a, option_b, option_c, option_d, correct_option, created_at
		 FROM exam_questions WHERE exam_id = $1 ORDER BY id`,
		examID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	var questions []models.ExamQuestion
	for rows.Next() {
		var q models.ExamQuestion
		_ = rows.Scan(&q.ID, &q.ExamID, &q.QuestionText, &q.OptionA, &q.OptionB, &q.OptionC, &q.OptionD, &q.CorrectOption, &q.CreatedAt)
		questions = append(questions, q)
	}

	if questions == nil {
		questions = []models.ExamQuestion{}
	}

	c.JSON(http.StatusOK, questions)
}

func AdminAddExamQuestion(c *gin.Context) {
	examID := c.Param("id")
	if !teacherOwnsBatchRow(c, "exams", examID) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "not authorized for this batch"})
		return
	}
	var req models.ExamQuestionPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var q models.ExamQuestion
	err := database.DB.QueryRow(
		context.Background(),
		`INSERT INTO exam_questions (exam_id, question_text, option_a, option_b, option_c, option_d, correct_option)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, exam_id, question_text, option_a, option_b, option_c, option_d, correct_option, created_at`,
		examID, req.QuestionText, req.OptionA, req.OptionB, req.OptionC, req.OptionD, req.CorrectOption,
	).Scan(&q.ID, &q.ExamID, &q.QuestionText, &q.OptionA, &q.OptionB, &q.OptionC, &q.OptionD, &q.CorrectOption, &q.CreatedAt)

	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to add question"})
		return
	}

	_, _ = database.DB.Exec(
		context.Background(),
		`UPDATE exams SET total_questions = (SELECT COUNT(*) FROM exam_questions WHERE exam_id = $1) WHERE id = $1`,
		examID,
	)

	c.JSON(http.StatusCreated, q)
}

func AdminDeleteExamQuestion(c *gin.Context) {
	examID := c.Param("id")
	qID := c.Param("qid")
	if !teacherOwnsBatchRow(c, "exams", examID) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "not authorized for this batch"})
		return
	}

	tag, err := database.DB.Exec(
		context.Background(),
		`DELETE FROM exam_questions WHERE id = $1 AND exam_id = $2`, qID, examID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "question not found"})
		return
	}

	_, _ = database.DB.Exec(
		context.Background(),
		`UPDATE exams SET total_questions = (SELECT COUNT(*) FROM exam_questions WHERE exam_id = $1) WHERE id = $1`,
		examID,
	)

	c.JSON(http.StatusOK, models.SuccessResponse{Message: "question deleted"})
}

// ── Public handlers (no auth) ──────────────────────────────────────────────

func PublicGetCourses(c *gin.Context) {
	classLevel := c.Query("class_level")
	courseType := c.Query("type")

	query := `SELECT ` + courseColumns + ` FROM courses WHERE 1=1`
	var args []interface{}
	argIdx := 1

	if classLevel != "" && classLevel != "all" {
		query += ` AND class_level = $` + strconv.Itoa(argIdx)
		args = append(args, classLevel)
		argIdx++
	}
	if courseType != "" && courseType != "all" {
		query += ` AND type = $` + strconv.Itoa(argIdx)
		args = append(args, courseType)
		argIdx++
	}
	query += ` ORDER BY created_at DESC`

	rows, err := database.DB.Query(context.Background(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	var courses []models.Course
	for rows.Next() {
		co, _ := scanCourse(rows)
		courses = append(courses, co)
	}

	if courses == nil {
		courses = []models.Course{}
	}

	c.JSON(http.StatusOK, courses)
}

func PublicGetCourse(c *gin.Context) {
	id := c.Param("id")

	var course models.Course
	err := database.DB.QueryRow(
		context.Background(),
		`SELECT `+courseColumns+` FROM courses WHERE id = $1`, id,
	).Scan(
		&course.ID, &course.Title, &course.TitleBn, &course.Description, &course.Subject,
		&course.Teacher, &course.Instructors, &course.ClassLevel, &course.Type, &course.Schedule,
		&course.Color, &course.Gradient, &course.Price,
		&course.OldPrice, &course.Duration, &course.Badge, &course.StudentsCount,
		&course.ClassesCount, &course.ExamsCount, &course.Rating, &course.ReviewsCount,
		&course.Curriculum, &course.Features, &course.CreatedAt, &course.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "course not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}

	c.JSON(http.StatusOK, course)
}

func PublicGetExamQuestions(c *gin.Context) {
	examID := c.Param("id")

	rows, err := database.DB.Query(
		context.Background(),
		`SELECT id, exam_id, question_text, option_a, option_b, option_c, option_d, correct_option, created_at
		 FROM exam_questions WHERE exam_id = $1 ORDER BY id`,
		examID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	var questions []models.ExamQuestion
	for rows.Next() {
		var q models.ExamQuestion
		_ = rows.Scan(&q.ID, &q.ExamID, &q.QuestionText, &q.OptionA, &q.OptionB, &q.OptionC, &q.OptionD, &q.CorrectOption, &q.CreatedAt)
		questions = append(questions, q)
	}

	if questions == nil {
		questions = []models.ExamQuestion{}
	}

	c.JSON(http.StatusOK, questions)
}

// ── Enrollment handlers ────────────────────────────────────────────────────

func PublicCreateEnrollment(c *gin.Context) {
	var req models.CreateEnrollmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	if req.PaymentMethod == "" {
		req.PaymentMethod = "manual"
	}

	var enrollment models.Enrollment
	err := database.DB.QueryRow(
		context.Background(),
		`INSERT INTO enrollments (course_id, full_name, mobile, payment_method, mobile_banking, amount, sent_from, sent_to, referral_source)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, course_id, full_name, mobile, payment_method, mobile_banking, amount, sent_from, sent_to, referral_source, status, created_at, updated_at`,
		req.CourseID, req.FullName, req.Mobile, req.PaymentMethod, req.MobileBanking,
		req.Amount, req.SentFrom, req.SentTo, req.ReferralSource,
	).Scan(&enrollment.ID, &enrollment.CourseID, &enrollment.FullName, &enrollment.Mobile,
		&enrollment.PaymentMethod, &enrollment.MobileBanking, &enrollment.Amount,
		&enrollment.SentFrom, &enrollment.SentTo, &enrollment.ReferralSource,
		&enrollment.Status, &enrollment.CreatedAt, &enrollment.UpdatedAt)

	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create enrollment"})
		return
	}

	_ = database.DB.QueryRow(context.Background(), `SELECT title FROM courses WHERE id = $1`, req.CourseID).Scan(&enrollment.CourseName)

	c.JSON(http.StatusCreated, enrollment)
}

func AdminGetEnrollments(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	status := c.Query("status")
	search := c.Query("search")
	userID := c.Query("user_id")

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	var total int
	var enrollments []models.Enrollment

	baseQuery := `FROM enrollments e LEFT JOIN courses c ON e.course_id = c.id`
	whereClause := ""
	args := []interface{}{}
	argIdx := 1

	if status != "" && status != "all" {
		whereClause += ` WHERE e.status = $` + strconv.Itoa(argIdx)
		args = append(args, status)
		argIdx++
	}
	if search != "" {
		if whereClause == "" {
			whereClause += " WHERE "
		} else {
			whereClause += " AND "
		}
		whereClause += `(e.full_name ILIKE $` + strconv.Itoa(argIdx) + ` OR e.mobile ILIKE $` + strconv.Itoa(argIdx) + ` OR c.title ILIKE $` + strconv.Itoa(argIdx) + `)`
		args = append(args, "%"+search+"%")
		argIdx++
	}
	if userID != "" {
		if whereClause == "" {
			whereClause += " WHERE "
		} else {
			whereClause += " AND "
		}
		whereClause += `e.user_id = $` + strconv.Itoa(argIdx)
		args = append(args, userID)
		argIdx++
	}

	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	_ = database.DB.QueryRow(context.Background(), `SELECT COUNT(*) `+baseQuery+whereClause, countArgs...).Scan(&total)

	queryArgs := make([]interface{}, len(args))
	copy(queryArgs, args)
	queryArgs = append(queryArgs, perPage, offset)
	rows, err := database.DB.Query(
		context.Background(),
		`SELECT e.id, COALESCE(e.course_id, 0), COALESCE(c.title,''), COALESCE(c.type,'online'), e.full_name, e.mobile, e.user_id, e.payment_method,
			e.mobile_banking, e.amount, e.sent_from, e.sent_to, e.referral_source, e.status, e.enrolled_by, e.batch_id, COALESCE(b.name,''), e.created_at, e.updated_at
		 `+baseQuery+` LEFT JOIN batches b ON e.batch_id = b.id`+whereClause+` ORDER BY e.created_at DESC LIMIT $`+strconv.Itoa(argIdx)+` OFFSET $`+strconv.Itoa(argIdx+1),
		queryArgs...,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	for rows.Next() {
		var en models.Enrollment
		_ = rows.Scan(&en.ID, &en.CourseID, &en.CourseName, &en.CourseType, &en.FullName, &en.Mobile, &en.UserID,
			&en.PaymentMethod, &en.MobileBanking, &en.Amount, &en.SentFrom, &en.SentTo,
			&en.ReferralSource, &en.Status, &en.EnrolledBy, &en.BatchID, &en.BatchName, &en.CreatedAt, &en.UpdatedAt)
		enrollments = append(enrollments, en)
	}

	if enrollments == nil {
		enrollments = []models.Enrollment{}
	}

	c.JSON(http.StatusOK, models.PaginatedEnrollments{
		Enrollments: enrollments,
		Total:       total,
		Page:        page,
		PerPage:     perPage,
		TotalPages:  int(math.Ceil(float64(total) / float64(perPage))),
	})
}

func AdminUpdateEnrollmentStatus(c *gin.Context) {
	id := c.Param("id")
	var req models.UpdateEnrollmentStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var enrollment models.Enrollment
	err := database.DB.QueryRow(
		context.Background(),
		`UPDATE enrollments SET status = $1, updated_at = NOW()
		 WHERE id = $2
		 RETURNING id, course_id, full_name, mobile, payment_method, mobile_banking, amount, sent_from, sent_to, referral_source, status, created_at, updated_at`,
		req.Status, id,
	).Scan(&enrollment.ID, &enrollment.CourseID, &enrollment.FullName, &enrollment.Mobile,
		&enrollment.PaymentMethod, &enrollment.MobileBanking, &enrollment.Amount,
		&enrollment.SentFrom, &enrollment.SentTo, &enrollment.ReferralSource,
		&enrollment.Status, &enrollment.CreatedAt, &enrollment.UpdatedAt)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "enrollment not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}

	_ = database.DB.QueryRow(context.Background(), `SELECT title FROM courses WHERE id = $1`, enrollment.CourseID).Scan(&enrollment.CourseName)

	c.JSON(http.StatusOK, enrollment)
}

func AdminDeleteEnrollment(c *gin.Context) {
	id := c.Param("id")
	tag, err := database.DB.Exec(context.Background(), `DELETE FROM enrollments WHERE id = $1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "enrollment not found"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "enrollment deleted"})
}

func AdminDirectEnroll(c *gin.Context) {
	var req models.DirectEnrollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var fullName, mobile string
	var userID *int

	if req.UserID > 0 {
		err := database.DB.QueryRow(context.Background(),
			`SELECT id, full_name, mobile FROM users WHERE id = $1`, req.UserID,
		).Scan(&userID, &fullName, &mobile)
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "user not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
			return
		}
		// The admin may have corrected the name on this form for an already
		// logged-in student — treat that edit as authoritative.
		if strings.TrimSpace(req.FullName) != "" {
			fullName = req.FullName
		}
	} else {
		mobile = req.Mobile

		// Check if user exists with this mobile
		var uid int
		err := database.DB.QueryRow(context.Background(),
			`SELECT id, full_name FROM users WHERE mobile = $1`, mobile,
		).Scan(&uid, &fullName)
		if err == nil {
			userID = &uid
			if strings.TrimSpace(req.FullName) != "" {
				fullName = req.FullName
			}
		} else if err == pgx.ErrNoRows {
			// User not found — auto-create a user account so they can log in via OTP
			name := req.FullName
			if name == "" {
				name = "Student"
			}
			var newUserID int
			err := database.DB.QueryRow(context.Background(),
				`INSERT INTO users (full_name, mobile, password_hash, verified, gender, student_class, school, shift, father_name, father_mobile, mother_name, mother_mobile, notification_mobile, address)
				 VALUES ($1, $2, '', TRUE, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
				 RETURNING id`, name, mobile, req.Gender, req.StudentClass, req.School, req.SchoolShift, req.FatherName, req.FatherMobile, req.MotherName, req.MotherMobile, req.NotificationMobile, req.Address,
			).Scan(&newUserID)
			if err == nil {
				userID = &newUserID
				fullName = name
			}
		}
	}

	// This form doubles as the profile editor for an already-registered
	// student, so whatever the admin enters here should overwrite the user's
	// stored profile — falling back to the existing value only when a field
	// is left blank, so we never wipe out data the admin didn't touch.
	if userID != nil {
		_, _ = database.DB.Exec(context.Background(),
			`UPDATE users SET
				full_name = COALESCE(NULLIF($2,''), full_name),
				gender = COALESCE(NULLIF($3,''), gender),
				student_class = COALESCE(NULLIF($4,''), student_class),
				school = COALESCE(NULLIF($5,''), school),
				shift = COALESCE(NULLIF($6,''), shift),
				father_name = COALESCE(NULLIF($7,''), father_name),
				father_mobile = COALESCE(NULLIF($8,''), father_mobile),
				mother_name = COALESCE(NULLIF($9,''), mother_name),
				mother_mobile = COALESCE(NULLIF($10,''), mother_mobile),
				notification_mobile = COALESCE(NULLIF($11,''), notification_mobile),
				address = COALESCE(NULLIF($12,''), address)
			 WHERE id = $1`,
			*userID, req.FullName, req.Gender, req.StudentClass, req.School, req.SchoolShift, req.FatherName, req.FatherMobile, req.MotherName, req.MotherMobile, req.NotificationMobile, req.Address)
	}

	paymentMethod := req.PaymentMethod
	if paymentMethod == "" {
		paymentMethod = "cash"
	}

	studentID := strings.TrimSpace(req.StudentID)
	if studentID == "" && req.BatchID != nil {
		generated, err := nextStudentIDForBatch(context.Background(), *req.BatchID)
		if err == nil {
			studentID = generated
		}
	}
	if studentID != "" {
		exists, err := studentIDExists(context.Background(), studentID, 0)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
			return
		}
		if exists {
			c.JSON(http.StatusConflict, models.ErrorResponse{Error: "student ID is already in use"})
			return
		}
	}

	var enrollment models.Enrollment
	err := database.DB.QueryRow(
		context.Background(),
		`INSERT INTO enrollments (course_id, full_name, mobile, student_id, user_id, batch_id, amount, payment_method, sent_from, status, enrolled_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'approved', 'staff')
		 RETURNING id, COALESCE(course_id, 0), full_name, mobile, student_id, user_id, batch_id, amount, payment_method, COALESCE(sent_from, ''), status, enrolled_by, created_at, updated_at`,
		nullableCourseID(req.CourseID), fullName, mobile, studentID, userID, req.BatchID, req.Amount, paymentMethod, req.Reference,
	).Scan(&enrollment.ID, &enrollment.CourseID, &enrollment.FullName, &enrollment.Mobile, &enrollment.StudentID,
		&enrollment.UserID, &enrollment.BatchID, &enrollment.Amount, &enrollment.PaymentMethod, &enrollment.SentFrom,
		&enrollment.Status, &enrollment.EnrolledBy, &enrollment.CreatedAt, &enrollment.UpdatedAt)

	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create enrollment"})
		return
	}

	_ = database.DB.QueryRow(context.Background(), `SELECT title FROM courses WHERE id = $1`, req.CourseID).Scan(&enrollment.CourseName)

	// Direct enrollment means the student paid the collecting staff on the spot,
	// so record it as an already-verified payment (not the pending default) —
	// otherwise the amount only lives on the enrollment row and never shows up
	// in the Payments ledger.
	if req.Amount > 0 && userID != nil {
		adminID, _ := c.Get("admin_id")
		receiptNo := fmt.Sprintf("EDU-%d-%04d", time.Now().Unix()%100000, time.Now().UnixNano()%10000)
		_, _ = database.DB.Exec(context.Background(),
			`INSERT INTO payments (user_id, enrollment_id, course_id, amount, method, sender_number, receipt_number, status, verified_by, verified_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, 'verified', $8, NOW())`,
			*userID, enrollment.ID, nullableCourseID(req.CourseID), req.Amount, paymentMethod, req.Reference, receiptNo, adminID,
		)
	}

	c.JSON(http.StatusCreated, enrollment)
}

func AdminSearchUsers(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		c.JSON(http.StatusOK, []models.User{})
		return
	}

	rows, err := database.DB.Query(context.Background(),
		`SELECT u.id, u.full_name, u.mobile, COALESCE(u.student_class,''), COALESCE(u.father_name,''), COALESCE(u.father_mobile,''), u.verified,
		 COALESCE(array_agg(b.name) FILTER (WHERE b.name IS NOT NULL), '{}')
		 FROM users u
		 LEFT JOIN enrollments e ON e.user_id = u.id AND e.status = 'approved' AND e.batch_id IS NOT NULL
		 LEFT JOIN batches b ON b.id = e.batch_id
		 WHERE u.mobile ILIKE $1 OR u.full_name ILIKE $1
		 GROUP BY u.id
		 ORDER BY u.full_name LIMIT 20`, "%"+q+"%")
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	type UserResult struct {
		ID           int      `json:"id"`
		FullName     string   `json:"full_name"`
		Mobile       string   `json:"mobile"`
		StudentClass string   `json:"student_class"`
		FatherName   string   `json:"father_name"`
		FatherMobile string   `json:"father_mobile"`
		Verified     bool     `json:"verified"`
		Batches      []string `json:"batches"`
	}
	var users []UserResult
	for rows.Next() {
		var u UserResult
		if err := rows.Scan(&u.ID, &u.FullName, &u.Mobile, &u.StudentClass, &u.FatherName, &u.FatherMobile, &u.Verified, &u.Batches); err == nil {
			users = append(users, u)
		}
	}
	if users == nil {
		users = []UserResult{}
	}
	c.JSON(http.StatusOK, users)
}

func AdminGetBatchStudents(c *gin.Context) {
	batchID := c.Param("id")
	rows, err := database.DB.Query(context.Background(),
		`SELECT e.id, COALESCE(e.course_id, 0), COALESCE(c.title,''), e.full_name, e.mobile, e.student_id, e.user_id,
		 e.amount, e.enrolled_by, e.batch_id, COALESCE(b.name,''), e.created_at::text
		 FROM enrollments e
		 LEFT JOIN courses c ON e.course_id = c.id
		 LEFT JOIN batches b ON e.batch_id = b.id
		 WHERE e.batch_id = $1 AND e.status = 'approved'
		 ORDER BY e.created_at DESC`, batchID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	type BatchStudent struct {
		ID         int    `json:"id"`
		CourseID   int    `json:"course_id"`
		CourseName string `json:"course_name"`
		FullName   string `json:"full_name"`
		Mobile     string `json:"mobile"`
		StudentID  string `json:"student_id"`
		UserID     *int   `json:"user_id"`
		Amount     int    `json:"amount"`
		EnrolledBy string `json:"enrolled_by"`
		BatchID    *int   `json:"batch_id"`
		BatchName  string `json:"batch_name"`
		CreatedAt  string `json:"created_at"`
	}
	var students []BatchStudent
	for rows.Next() {
		var s BatchStudent
		if err := rows.Scan(&s.ID, &s.CourseID, &s.CourseName, &s.FullName, &s.Mobile, &s.StudentID, &s.UserID,
			&s.Amount, &s.EnrolledBy, &s.BatchID, &s.BatchName, &s.CreatedAt); err != nil {
			log.Printf("Error scanning batch student: %v", err)
			continue
		}
		students = append(students, s)
	}
	if students == nil {
		students = []BatchStudent{}
	}
	c.JSON(http.StatusOK, students)
}

// ========== ADMIN MANAGEMENT (master_admin only) ==========

func AdminListAdmins(c *gin.Context) {
	rows, err := database.DB.Query(context.Background(),
		`SELECT id, email, full_name, COALESCE(role, 'admin'), created_at, updated_at
		 FROM admin_users ORDER BY id ASC`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	var admins []models.AdminUser
	for rows.Next() {
		var a models.AdminUser
		if err := rows.Scan(&a.ID, &a.Email, &a.FullName, &a.Role, &a.CreatedAt, &a.UpdatedAt); err != nil {
			continue
		}
		admins = append(admins, a)
	}
	if admins == nil {
		admins = []models.AdminUser{}
	}
	c.JSON(http.StatusOK, admins)
}

func AdminCreateAdmin(c *gin.Context) {
	var req models.CreateAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	validRoles := map[string]bool{"admin": true, "moderator": true}
	if !validRoles[req.Role] {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid role: must be admin or moderator"})
		return
	}

	// Check duplicate email
	var count int
	_ = database.DB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM admin_users WHERE email = $1`, req.Email,
	).Scan(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, models.ErrorResponse{Error: "email already exists"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to hash password"})
		return
	}

	var admin models.AdminUser
	err = database.DB.QueryRow(context.Background(),
		`INSERT INTO admin_users (email, password_hash, full_name, role)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, email, full_name, COALESCE(role, 'admin'), created_at, updated_at`,
		req.Email, string(hash), req.FullName, req.Role,
	).Scan(&admin.ID, &admin.Email, &admin.FullName, &admin.Role, &admin.CreatedAt, &admin.UpdatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create admin"})
		return
	}
	c.JSON(http.StatusCreated, admin)
}

func AdminUpdateAdminRole(c *gin.Context) {
	id := c.Param("id")
	email := c.GetString("admin_email")

	var req struct {
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	validRoles := map[string]bool{"admin": true, "moderator": true}
	if !validRoles[req.Role] {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid role: must be admin or moderator"})
		return
	}

	// Can't change own role
	var targetEmail string
	_ = database.DB.QueryRow(context.Background(),
		`SELECT email FROM admin_users WHERE id = $1`, id,
	).Scan(&targetEmail)
	if targetEmail == email {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "cannot change your own role"})
		return
	}

	tag, err := database.DB.Exec(context.Background(),
		`UPDATE admin_users SET role = $1, updated_at = NOW() WHERE id = $2`, req.Role, id,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "admin not found"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "role updated"})
}

func AdminDeleteAdmin(c *gin.Context) {
	id := c.Param("id")
	email := c.GetString("admin_email")

	// Can't delete self
	var targetEmail string
	_ = database.DB.QueryRow(context.Background(),
		`SELECT email FROM admin_users WHERE id = $1`, id,
	).Scan(&targetEmail)
	if targetEmail == email {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "cannot delete your own account"})
		return
	}

	tag, err := database.DB.Exec(context.Background(), `DELETE FROM admin_users WHERE id = $1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "admin not found"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "admin deleted"})
}

func AdminResetPassword(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Password string `json:"password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to hash password"})
		return
	}

	tag, err := database.DB.Exec(context.Background(),
		`UPDATE admin_users SET password_hash = $1, updated_at = NOW() WHERE id = $2`, string(hash), id,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "admin not found"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "password reset successfully"})
}
