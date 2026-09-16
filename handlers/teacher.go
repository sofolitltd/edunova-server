package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"edunova-server/database"
	"edunova-server/middleware"
	"edunova-server/models"
)

// teacherAssignedBatchIDs returns the batch IDs assigned to the calling
// teacher, and whether the caller is a teacher at all. Admins get
// (nil, false) — unrestricted. A teacher with no batches yet gets
// ([]int{}, true) — callers must treat that as "show nothing", not "show
// everything", since an empty SQL IN/ANY() list would otherwise match all
// rows unless explicitly checked.
func teacherAssignedBatchIDs(c *gin.Context) ([]int, bool) {
	actorType, _ := c.Get("actor_type")
	if actorType != "teacher" {
		return nil, false
	}
	teacherID, _ := c.Get("teacher_id")

	rows, err := database.DB.Query(context.Background(),
		`SELECT batch_id FROM batch_teachers WHERE teacher_id = $1`, teacherID)
	if err != nil {
		return []int{}, true
	}
	defer rows.Close()

	ids := []int{}
	for rows.Next() {
		var id int
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	return ids, true
}

// teacherOwnsDoubt reports whether the calling teacher is allowed to act on
// the given doubt — true for admins (unrestricted) and for a teacher whose
// assigned batches include the doubt's student.
func teacherOwnsDoubt(c *gin.Context, doubtID string) bool {
	teacherIDs, isTeacher := teacherAssignedBatchIDs(c)
	if !isTeacher {
		return true
	}
	if len(teacherIDs) == 0 {
		return false
	}
	var count int
	database.DB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM doubts d
		 JOIN enrollments e ON e.user_id = d.student_id AND e.status = 'approved'
		 WHERE d.id = $1 AND e.batch_id = ANY($2)`, doubtID, teacherIDs).Scan(&count)
	return count > 0
}

// teacherCanUseBatchID reports whether the calling teacher may create/edit a
// row scoped to the given batch id. Admins are always allowed; a teacher may
// only use one of their own assigned batches (0/unset is disallowed for a
// teacher, since an unscoped row would be invisible to every batch filter).
func teacherCanUseBatchID(c *gin.Context, batchID int) bool {
	teacherIDs, isTeacher := teacherAssignedBatchIDs(c)
	if !isTeacher {
		return true
	}
	if batchID == 0 {
		return false
	}
	for _, id := range teacherIDs {
		if id == batchID {
			return true
		}
	}
	return false
}

// teacherOwnsBatchRow reports whether the calling teacher may act on an
// existing row in `table` identified by `id`, based on that table's own
// batch_id column. Admins are always allowed.
func teacherOwnsBatchRow(c *gin.Context, table, id string) bool {
	teacherIDs, isTeacher := teacherAssignedBatchIDs(c)
	if !isTeacher {
		return true
	}
	if len(teacherIDs) == 0 {
		return false
	}
	var count int
	database.DB.QueryRow(context.Background(),
		fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE id = $1 AND batch_id = ANY($2)`, table),
		id, teacherIDs).Scan(&count)
	return count > 0
}

// teacherCanUseStudentID reports whether the calling teacher may act on the
// given student (by user id) — true for admins, and for a teacher only when
// that student has an approved enrollment in one of the teacher's batches.
func teacherCanUseStudentID(c *gin.Context, userID int) bool {
	teacherIDs, isTeacher := teacherAssignedBatchIDs(c)
	if !isTeacher {
		return true
	}
	if len(teacherIDs) == 0 || userID == 0 {
		return false
	}
	var count int
	database.DB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM enrollments WHERE user_id = $1 AND status = 'approved' AND batch_id = ANY($2)`,
		userID, teacherIDs).Scan(&count)
	return count > 0
}

// teacherOwnsResult reports whether the calling teacher may act on an
// existing student_results row, based on the row's own student's batch
// enrollment.
func teacherOwnsResult(c *gin.Context, resultID string) bool {
	teacherIDs, isTeacher := teacherAssignedBatchIDs(c)
	if !isTeacher {
		return true
	}
	if len(teacherIDs) == 0 {
		return false
	}
	var count int
	database.DB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM student_results r
		 JOIN enrollments e ON e.user_id = r.user_id AND e.status = 'approved'
		 WHERE r.id = $1 AND e.batch_id = ANY($2)`, resultID, teacherIDs).Scan(&count)
	return count > 0
}

func intSliceContains(ids []int, target string) bool {
	for _, id := range ids {
		if strconv.Itoa(id) == target {
			return true
		}
	}
	return false
}

// ========== TEACHER SELF-SERVICE (independent from admin_users) ==========

func TeacherLogin(c *gin.Context) {
	var req models.TeacherLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var teacher models.Teacher
	err := database.DB.QueryRow(
		context.Background(),
		`SELECT id, email, password_hash, full_name, created_at, updated_at
		 FROM teachers WHERE email = $1`,
		req.Email,
	).Scan(&teacher.ID, &teacher.Email, &teacher.PasswordHash, &teacher.FullName, &teacher.CreatedAt, &teacher.UpdatedAt)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "invalid email or password"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(teacher.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "invalid email or password"})
		return
	}

	token, err := middleware.GenerateTeacherToken(teacher.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, models.TeacherAuthResponse{Token: token, Teacher: teacher})
}

func TeacherGetProfile(c *gin.Context) {
	email := c.GetString("teacher_email")

	var teacher models.Teacher
	err := database.DB.QueryRow(context.Background(),
		`SELECT id, email, full_name, created_at, updated_at FROM teachers WHERE email = $1`, email,
	).Scan(&teacher.ID, &teacher.Email, &teacher.FullName, &teacher.CreatedAt, &teacher.UpdatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "teacher not found"})
		return
	}
	c.JSON(http.StatusOK, teacher)
}

func TeacherUpdateProfile(c *gin.Context) {
	email := c.GetString("teacher_email")

	var req models.UpdateTeacherProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var count int
	_ = database.DB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM teachers WHERE email = $1 AND email != $2`, req.Email, email,
	).Scan(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, models.ErrorResponse{Error: "email already taken"})
		return
	}

	var teacher models.Teacher
	err := database.DB.QueryRow(context.Background(),
		`UPDATE teachers SET full_name=$1, email=$2, updated_at=NOW()
		 WHERE email=$3
		 RETURNING id, email, full_name, created_at, updated_at`,
		req.FullName, req.Email, email,
	).Scan(&teacher.ID, &teacher.Email, &teacher.FullName, &teacher.CreatedAt, &teacher.UpdatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update profile"})
		return
	}
	c.JSON(http.StatusOK, teacher)
}

func TeacherChangePassword(c *gin.Context) {
	email := c.GetString("teacher_email")

	var req models.ChangeTeacherPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var passwordHash string
	err := database.DB.QueryRow(context.Background(),
		`SELECT password_hash FROM teachers WHERE email = $1`, email,
	).Scan(&passwordHash)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "teacher not found"})
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
		`UPDATE teachers SET password_hash=$1, updated_at=NOW() WHERE email=$2`,
		string(newHash), email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to change password"})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{Message: "password changed successfully"})
}

// ========== ADMIN-SIDE TEACHER MANAGEMENT (admin & master_admin) ==========

func AdminListTeachers(c *gin.Context) {
	rows, err := database.DB.Query(context.Background(),
		`SELECT id, email, full_name, created_at, updated_at FROM teachers ORDER BY id ASC`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	var teachers []models.Teacher
	for rows.Next() {
		var t models.Teacher
		if err := rows.Scan(&t.ID, &t.Email, &t.FullName, &t.CreatedAt, &t.UpdatedAt); err != nil {
			continue
		}
		teachers = append(teachers, t)
	}
	if teachers == nil {
		teachers = []models.Teacher{}
	}
	c.JSON(http.StatusOK, teachers)
}

func AdminCreateTeacher(c *gin.Context) {
	var req models.CreateTeacherRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var count int
	_ = database.DB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM teachers WHERE email = $1`, req.Email,
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

	var teacher models.Teacher
	err = database.DB.QueryRow(context.Background(),
		`INSERT INTO teachers (email, password_hash, full_name)
		 VALUES ($1, $2, $3)
		 RETURNING id, email, full_name, created_at, updated_at`,
		req.Email, string(hash), req.FullName,
	).Scan(&teacher.ID, &teacher.Email, &teacher.FullName, &teacher.CreatedAt, &teacher.UpdatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create teacher"})
		return
	}
	c.JSON(http.StatusCreated, teacher)
}

func AdminDeleteTeacher(c *gin.Context) {
	id := c.Param("id")

	tag, err := database.DB.Exec(context.Background(), `DELETE FROM teachers WHERE id = $1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "teacher not found"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "teacher deleted"})
}

// ========== BATCH <-> TEACHER ASSIGNMENT ==========

func AdminGetBatchTeachers(c *gin.Context) {
	batchID := c.Param("id")

	rows, err := database.DB.Query(context.Background(),
		`SELECT t.id, t.email, t.full_name
		 FROM batch_teachers bt
		 JOIN teachers t ON bt.teacher_id = t.id
		 WHERE bt.batch_id = $1
		 ORDER BY t.full_name ASC`, batchID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	var teachers []models.BatchTeacher
	for rows.Next() {
		var t models.BatchTeacher
		if err := rows.Scan(&t.ID, &t.Email, &t.FullName); err != nil {
			continue
		}
		teachers = append(teachers, t)
	}
	if teachers == nil {
		teachers = []models.BatchTeacher{}
	}
	c.JSON(http.StatusOK, teachers)
}

func AdminAssignBatchTeacher(c *gin.Context) {
	batchID := c.Param("id")

	var req models.AssignBatchTeacherRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	_, err := database.DB.Exec(context.Background(),
		`INSERT INTO batch_teachers (batch_id, teacher_id) VALUES ($1, $2)
		 ON CONFLICT (batch_id, teacher_id) DO NOTHING`,
		batchID, req.TeacherID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to assign teacher"})
		return
	}
	c.JSON(http.StatusCreated, models.SuccessResponse{Message: "teacher assigned"})
}

func AdminUnassignBatchTeacher(c *gin.Context) {
	batchID := c.Param("id")
	teacherID := c.Param("teacherId")

	tag, err := database.DB.Exec(context.Background(),
		`DELETE FROM batch_teachers WHERE batch_id = $1 AND teacher_id = $2`, batchID, teacherID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "assignment not found"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "teacher unassigned"})
}

// TeacherGetMyBatches returns the batches assigned to the authenticated
// teacher, for their portal dashboard.
func TeacherGetMyBatches(c *gin.Context) {
	teacherID, _ := c.Get("teacher_id")

	rows, err := database.DB.Query(context.Background(),
		`SELECT b.id, b.name, b.class_level, COALESCE(b.shift, ''), COALESCE(b.code, ''), COALESCE(b.schedule, ''),
		 (SELECT COUNT(*) FROM enrollments e WHERE e.batch_id = b.id AND e.status = 'approved')
		 FROM batch_teachers bt
		 JOIN batches b ON bt.batch_id = b.id
		 WHERE bt.teacher_id = $1
		 ORDER BY b.class_level, b.name`, teacherID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	var batches []models.TeacherBatch
	for rows.Next() {
		var b models.TeacherBatch
		if err := rows.Scan(&b.ID, &b.Name, &b.ClassLevel, &b.Shift, &b.Code, &b.Schedule, &b.StudentCount); err != nil {
			continue
		}
		batches = append(batches, b)
	}
	if batches == nil {
		batches = []models.TeacherBatch{}
	}
	c.JSON(http.StatusOK, batches)
}

func AdminResetTeacherPassword(c *gin.Context) {
	id := c.Param("id")

	var req models.ResetTeacherPasswordRequest
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
		`UPDATE teachers SET password_hash = $1, updated_at = NOW() WHERE id = $2`, string(hash), id,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "teacher not found"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "password reset successfully"})
}
