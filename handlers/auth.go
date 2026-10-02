package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"edunova-server/database"
	"edunova-server/middleware"
	"edunova-server/models"
	"edunova-server/services"
)

func Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to hash password"})
		return
	}

	var user models.User
	err = database.DB.QueryRow(
		context.Background(),
		`INSERT INTO users (full_name, mobile, password_hash, student_class)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, full_name, mobile, verified, created_at, updated_at`,
		req.FullName, req.Mobile, string(hash), req.StudentClass,
	).Scan(&user.ID, &user.FullName, &user.Mobile, &user.Verified, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		c.JSON(http.StatusConflict, models.ErrorResponse{Error: "mobile number already registered"})
		return
	}

	// TODO: Uncomment below to activate OTP verification in production
	// code := generateOTP()
	// expiresAt := time.Now().Add(otpExpiry)
	// _, _ = database.DB.Exec(
	// 	context.Background(),
	// 	`INSERT INTO otps (mobile, code, expires_at) VALUES ($1, $2, $3)`,
	// 	user.Mobile, code, expiresAt,
	// )
	// message := fmt.Sprintf("Your EduNova verification code is: %s. It will expire in 2 minutes.", code)
	// services.SendSMS(user.Mobile, message)

	// Auto-verify in dev mode (OTP disabled)
	_, _ = database.DB.Exec(
		context.Background(),
		`UPDATE users SET verified = TRUE, updated_at = NOW() WHERE mobile = $1`,
		user.Mobile,
	)

	c.JSON(http.StatusCreated, models.SuccessResponse{
		Message: fmt.Sprintf("Registration successful. OTP skipped (dev mode). %s", user.Mobile),
	})
}

func Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var user models.User
	err := database.DB.QueryRow(
		context.Background(),
		`SELECT id, full_name, mobile, password_hash, verified,
		 COALESCE(father_name,''), COALESCE(father_mobile,''),
		 COALESCE(mother_name,''), COALESCE(mother_mobile,''),
		 COALESCE(notification_mobile,''), COALESCE(gender,''),
		 COALESCE(religion,''), COALESCE(student_class,''),
		 COALESCE(shift,''), COALESCE(school,''),
		 COALESCE(present_address,''), COALESCE(permanent_address,''),
		 created_at, updated_at
		 FROM users WHERE mobile = $1`,
		req.Mobile,
	).Scan(&user.ID, &user.FullName, &user.Mobile, &user.PasswordHash, &user.Verified,
		&user.FatherName, &user.FatherMobile,
		&user.MotherName, &user.MotherMobile,
		&user.NotificationMobile, &user.Gender,
		&user.Religion, &user.StudentClass,
		&user.Shift, &user.School, &user.PresentAddress, &user.PermanentAddress,
		&user.CreatedAt, &user.UpdatedAt)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "invalid mobile or password"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "invalid mobile or password"})
		return
	}

	token, err := middleware.GenerateToken(user.Mobile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, models.AuthResponse{
		Token: token,
		User:  user,
	})
}

func GetUser(c *gin.Context) {
	mobile := c.GetString("mobile")

	var user models.User
	err := database.DB.QueryRow(
		context.Background(),
		`SELECT id, full_name, mobile, verified,
		 COALESCE(father_name,''), COALESCE(father_mobile,''),
		 COALESCE(mother_name,''), COALESCE(mother_mobile,''),
		 COALESCE(notification_mobile,''), COALESCE(gender,''),
		 COALESCE(religion,''), COALESCE(student_class,''),
		 COALESCE(shift,''), COALESCE(school,''),
		 COALESCE(present_address,''), COALESCE(permanent_address,''),
		 created_at, updated_at
		 FROM users WHERE mobile = $1`,
		mobile,
	).Scan(&user.ID, &user.FullName, &user.Mobile, &user.Verified,
		&user.FatherName, &user.FatherMobile,
		&user.MotherName, &user.MotherMobile,
		&user.NotificationMobile, &user.Gender,
		&user.Religion, &user.StudentClass,
		&user.Shift, &user.School, &user.PresentAddress, &user.PermanentAddress,
		&user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "user not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}

func ChangePassword(c *gin.Context) {
	mobile := c.GetString("mobile")

	var req models.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var passwordHash string
	err := database.DB.QueryRow(
		context.Background(),
		`SELECT password_hash FROM users WHERE mobile = $1`,
		mobile,
	).Scan(&passwordHash)

	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "user not found"})
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

	_, err = database.DB.Exec(
		context.Background(),
		`UPDATE users SET password_hash = $1, updated_at = NOW() WHERE mobile = $2`,
		string(newHash), mobile,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update password"})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{Message: "password changed successfully"})
}

func UpdateUserProfile(c *gin.Context) {
	mobile := c.GetString("mobile")

	var req models.UpdateUserProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var user models.User
	err := database.DB.QueryRow(context.Background(),
		`UPDATE users SET full_name=$1, father_name=$2, father_mobile=$3,
		 mother_name=$4, mother_mobile=$5, notification_mobile=$6,
		 gender=$7, religion=$8, student_class=$9, shift=$10,
		 school=$11, present_address=$12, permanent_address=$13, updated_at=NOW() WHERE mobile=$14
		 RETURNING id, full_name, mobile, verified,
		 COALESCE(father_name,''), COALESCE(father_mobile,''),
		 COALESCE(mother_name,''), COALESCE(mother_mobile,''),
		 COALESCE(notification_mobile,''), COALESCE(gender,''),
		 COALESCE(religion,''), COALESCE(student_class,''),
		 COALESCE(shift,''), COALESCE(school,''),
		 COALESCE(present_address,''), COALESCE(permanent_address,''),
		 created_at, updated_at`,
		req.FullName, req.FatherName, req.FatherMobile,
		req.MotherName, req.MotherMobile, req.NotificationMobile,
		req.Gender, req.Religion, req.StudentClass, req.Shift,
		req.School, req.PresentAddress, req.PermanentAddress, mobile,
	).Scan(&user.ID, &user.FullName, &user.Mobile, &user.Verified,
		&user.FatherName, &user.FatherMobile,
		&user.MotherName, &user.MotherMobile,
		&user.NotificationMobile, &user.Gender,
		&user.Religion, &user.StudentClass,
		&user.Shift, &user.School, &user.PresentAddress, &user.PermanentAddress,
		&user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update profile"})
		return
	}
	c.JSON(http.StatusOK, user)
}

func GetUserEnrollments(c *gin.Context) {
	mobile := c.GetString("mobile")

	rows, err := database.DB.Query(context.Background(),
		`SELECT e.id, COALESCE(e.course_id, 0), COALESCE(c.title,''), COALESCE(c.type,'online'), e.full_name, e.mobile, e.user_id,
		 e.payment_method, e.mobile_banking, e.amount, e.sent_from, e.sent_to,
		 e.referral_source, e.status, e.enrolled_by, e.batch_id, COALESCE(b.name,''), COALESCE(b.schedule,''), e.created_at, e.updated_at
		 FROM enrollments e
		 LEFT JOIN courses c ON e.course_id = c.id
		 LEFT JOIN batches b ON e.batch_id = b.id
		 WHERE e.mobile = $1
		 ORDER BY e.created_at DESC`, mobile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch enrollments"})
		return
	}
	defer rows.Close()

	var enrollments []models.Enrollment
	for rows.Next() {
		var e models.Enrollment
		if err := rows.Scan(&e.ID, &e.CourseID, &e.CourseName, &e.CourseType, &e.FullName, &e.Mobile, &e.UserID,
			&e.PaymentMethod, &e.MobileBanking, &e.Amount, &e.SentFrom, &e.SentTo,
			&e.ReferralSource, &e.Status, &e.EnrolledBy, &e.BatchID, &e.BatchName, &e.BatchSchedule, &e.CreatedAt, &e.UpdatedAt); err != nil {
			continue
		}
		enrollments = append(enrollments, e)
	}
	if enrollments == nil {
		enrollments = []models.Enrollment{}
	}
	c.JSON(http.StatusOK, enrollments)
}

func GetUserFreeCourses(c *gin.Context) {
	rows, err := database.DB.Query(context.Background(),
		`SELECT `+`id, title, title_bn, description, subject, teacher, instructors, class_level, type, schedule, color, gradient,
		price, old_price, duration, badge, students_count, classes_count, exams_count,
		rating, reviews_count, curriculum, features, created_at, updated_at
		 FROM courses WHERE type = 'free' ORDER BY created_at DESC`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch free courses"})
		return
	}
	defer rows.Close()

	var courses []models.Course
	for rows.Next() {
		var co models.Course
		if err := rows.Scan(
			&co.ID, &co.Title, &co.TitleBn, &co.Description, &co.Subject, &co.Teacher,
			&co.Instructors, &co.ClassLevel, &co.Type, &co.Schedule, &co.Color, &co.Gradient,
			&co.Price, &co.OldPrice, &co.Duration, &co.Badge, &co.StudentsCount,
			&co.ClassesCount, &co.ExamsCount, &co.Rating, &co.ReviewsCount,
			&co.Curriculum, &co.Features, &co.CreatedAt, &co.UpdatedAt,
		); err != nil {
			continue
		}
		courses = append(courses, co)
	}
	if courses == nil {
		courses = []models.Course{}
	}
	c.JSON(http.StatusOK, courses)
}

func GetUserDashboardStats(c *gin.Context) {
	mobile := c.GetString("mobile")

	var totalEnrollments, pendingEnrollments, approvedEnrollments int
	_ = database.DB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM enrollments WHERE mobile=$1`, mobile).Scan(&totalEnrollments)
	_ = database.DB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM enrollments WHERE mobile=$1 AND status='pending'`, mobile).Scan(&pendingEnrollments)
	_ = database.DB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM enrollments WHERE mobile=$1 AND status='approved'`, mobile).Scan(&approvedEnrollments)

	var totalExams, completedExams int
	_ = database.DB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM exam_results er JOIN users u ON er.user_id=u.id WHERE u.mobile=$1`, mobile).Scan(&completedExams)
	_ = database.DB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM exams`).Scan(&totalExams)

	c.JSON(http.StatusOK, gin.H{
		"total_enrollments":    totalEnrollments,
		"pending_enrollments":  pendingEnrollments,
		"approved_enrollments": approvedEnrollments,
		"total_exams":          totalExams,
		"completed_exams":      completedExams,
	})
}

// OTPLogin sends an OTP to an existing user's mobile (passwordless login)
func OTPLogin(c *gin.Context) {
	var req models.OTPLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	// Check user exists
	var exists bool
	_ = database.DB.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM users WHERE mobile=$1)`, req.Mobile).Scan(&exists)

	if !exists {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "no account found with this mobile number"})
		return
	}

	code := generateOTP()
	expiresAt := time.Now().Add(otpExpiry)
	_, _ = database.DB.Exec(
		context.Background(),
		`INSERT INTO otps (mobile, code, expires_at) VALUES ($1, $2, $3)`,
		req.Mobile, code, expiresAt,
	)

	message := fmt.Sprintf("Your EduNova login code is: %s. It will expire in 2 minutes.", code)
	services.SendSMS(req.Mobile, message)

	c.JSON(http.StatusOK, models.SuccessResponse{
		Message: fmt.Sprintf("OTP sent to %s", req.Mobile),
	})
}

// OTPLoginVerify verifies the OTP and returns a JWT
func OTPLoginVerify(c *gin.Context) {
	var req models.OTPLoginVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var otp models.OTP
	err := database.DB.QueryRow(
		context.Background(),
		`SELECT id, mobile, code, expires_at, created_at
		 FROM otps
		 WHERE mobile = $1 AND code = $2
		 ORDER BY created_at DESC
		 LIMIT 1`,
		req.Mobile, req.Code,
	).Scan(&otp.ID, &otp.Mobile, &otp.Code, &otp.ExpiresAt, &otp.CreatedAt)

	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid OTP"})
		return
	}

	if time.Now().After(otp.ExpiresAt) {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "OTP expired"})
		return
	}

	// Delete used OTP
	_, _ = database.DB.Exec(context.Background(),
		`DELETE FROM otps WHERE mobile=$1`, req.Mobile)

	// Fetch user and return token
	var user models.User
	err = database.DB.QueryRow(context.Background(),
		`SELECT id, full_name, mobile, verified,
		 COALESCE(father_name,''), COALESCE(father_mobile,''),
		 COALESCE(mother_name,''), COALESCE(mother_mobile,''),
		 COALESCE(notification_mobile,''), COALESCE(gender,''),
		 COALESCE(religion,''), COALESCE(student_class,''),
		 COALESCE(shift,''), COALESCE(school,''),
		 COALESCE(present_address,''), COALESCE(permanent_address,''),
		 created_at, updated_at
		 FROM users WHERE mobile=$1`, req.Mobile,
	).Scan(&user.ID, &user.FullName, &user.Mobile, &user.Verified,
		&user.FatherName, &user.FatherMobile,
		&user.MotherName, &user.MotherMobile,
		&user.NotificationMobile, &user.Gender,
		&user.Religion, &user.StudentClass,
		&user.Shift, &user.School, &user.PresentAddress, &user.PermanentAddress,
		&user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "user not found"})
		return
	}

	token, err := middleware.GenerateToken(user.Mobile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, models.AuthResponse{Token: token, User: user})
}
