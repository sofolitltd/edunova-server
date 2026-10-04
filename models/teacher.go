package models

import "time"

type Teacher struct {
	ID           int       `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	FullName     string    `json:"full_name"`
	Nickname     string    `json:"nickname"`
	Gender       string    `json:"gender"`
	Phone        string    `json:"phone"`
	Education    string    `json:"education"`
	Bio          string    `json:"bio"`
	Address      string    `json:"address"`
	PhotoURL     string    `json:"photo_url"`
	JoinDate     *string   `json:"join_date"`
	LeaveDate    *string   `json:"leave_date"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type TeacherLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type TeacherAuthResponse struct {
	Token   string  `json:"token"`
	Teacher Teacher `json:"teacher"`
}

type CreateTeacherRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	FullName string `json:"full_name" binding:"required"`
	Nickname string `json:"nickname"`
	Gender   string `json:"gender" binding:"omitempty,oneof=male female"`
}

type UpdateTeacherProfileRequest struct {
	FullName string `json:"full_name" binding:"required,min=2"`
	Email    string `json:"email" binding:"required,email"`
}

// AdminUpdateTeacherRequest is the admin-side full profile edit, distinct
// from UpdateTeacherProfileRequest (the teacher's own self-service, which
// only touches full_name/email).
type AdminUpdateTeacherRequest struct {
	FullName  string  `json:"full_name" binding:"required,min=2"`
	Nickname  string  `json:"nickname"`
	Gender    string  `json:"gender" binding:"omitempty,oneof=male female"`
	Email     string  `json:"email" binding:"required,email"`
	Phone     string  `json:"phone"`
	Education string  `json:"education"`
	Bio       string  `json:"bio"`
	Address   string  `json:"address"`
	PhotoURL  string  `json:"photo_url"`
	JoinDate  *string `json:"join_date"`
	LeaveDate *string `json:"leave_date"`
}

type ChangeTeacherPasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

type ResetTeacherPasswordRequest struct {
	Password string `json:"password" binding:"required,min=6"`
}

// BatchTeacher is a teacher assigned to a batch, as seen from the batch side.
type BatchTeacher struct {
	ID       int    `json:"id"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`
}

type AssignBatchTeacherRequest struct {
	TeacherID int `json:"teacher_id" binding:"required"`
}

// TeacherBatch is a batch assigned to a teacher, as seen from the teacher's
// own dashboard/portal.
type TeacherBatch struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	ClassLevel   string `json:"class_level"`
	Shift        string `json:"shift"`
	Code         string `json:"code"`
	Schedule     string `json:"schedule"`
	StudentCount int    `json:"student_count"`
	CourseID     int    `json:"course_id"`
}

// TeacherDisplayNameEn is the English form used on the printable routine,
// e.g. "Tushar Sir" / "Ayesha Ma'am".
func TeacherDisplayNameEn(fullName, nickname, gender string) string {
	name := nickname
	if name == "" {
		name = fullName
	}
	switch gender {
	case "male":
		return name + " Sir"
	case "female":
		return name + " Ma'am"
	}
	return name
}

// TeacherDisplayName is how students address a teacher: the nickname (or full
// name when there is none) plus "স্যার"/"ম্যাম" by gender, e.g. "তুষার স্যার".
func TeacherDisplayName(fullName, nickname, gender string) string {
	name := nickname
	if name == "" {
		name = fullName
	}
	switch gender {
	case "male":
		return name + " স্যার"
	case "female":
		return name + " ম্যাম"
	}
	return name
}
