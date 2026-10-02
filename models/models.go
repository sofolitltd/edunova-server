package models

import (
	"time"
)

type User struct {
	ID                 int       `json:"id"`
	FullName           string    `json:"full_name"`
	Mobile             string    `json:"mobile"`
	PasswordHash       string    `json:"-"`
	Verified           bool      `json:"verified"`
	FatherName         string    `json:"father_name"`
	FatherMobile       string    `json:"father_mobile"`
	MotherName         string    `json:"mother_name"`
	MotherMobile       string    `json:"mother_mobile"`
	NotificationMobile string    `json:"notification_mobile"`
	Gender             string    `json:"gender"`
	Religion           string    `json:"religion"`
	StudentClass       string    `json:"student_class"`
	Shift              string    `json:"shift"`
	School             string    `json:"school"`
	PresentAddress     string    `json:"present_address"`
	PermanentAddress   string    `json:"permanent_address"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type OTP struct {
	ID        int       `json:"id"`
	Mobile    string    `json:"mobile"`
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type RegisterRequest struct {
	FullName     string `json:"full_name" binding:"required,min=3"`
	Mobile       string `json:"mobile" binding:"required,min=10"`
	Password     string `json:"password" binding:"required,min=6"`
	StudentClass string `json:"student_class" binding:"required,oneof=3 4 5 6 7 8"`
}

type LoginRequest struct {
	Mobile   string `json:"mobile" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type SuccessResponse struct {
	Message string `json:"message"`
}

type SendOTPRequest struct {
	Mobile string `json:"mobile" binding:"required"`
}

type VerifyOTPRequest struct {
	Mobile string `json:"mobile" binding:"required"`
	Code   string `json:"code" binding:"required"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

type OTPLoginRequest struct {
	Mobile string `json:"mobile" binding:"required"`
}

type OTPLoginVerifyRequest struct {
	Mobile string `json:"mobile" binding:"required"`
	Code   string `json:"code" binding:"required"`
}

type UpdateUserProfileRequest struct {
	FullName           string `json:"full_name"`
	FatherName         string `json:"father_name"`
	FatherMobile       string `json:"father_mobile"`
	MotherName         string `json:"mother_name"`
	MotherMobile       string `json:"mother_mobile"`
	NotificationMobile string `json:"notification_mobile"`
	Gender             string `json:"gender"`
	Religion           string `json:"religion"`
	StudentClass       string `json:"student_class" binding:"omitempty,oneof=3 4 5 6 7 8"`
	Shift              string `json:"shift"`
	School             string `json:"school"`
	PresentAddress     string `json:"present_address"`
	PermanentAddress   string `json:"permanent_address"`
}

type Attendance struct {
	ID           int       `json:"id"`
	StudentID    int       `json:"student_id"`
	StudentName  string    `json:"student_name"`
	Date         string    `json:"date"`
	Status       string    `json:"status"`
	MarkedBy     int       `json:"marked_by"`
	MarkedByName string    `json:"marked_by_name"`
	Notes        string    `json:"notes"`
	CreatedAt    time.Time `json:"created_at"`
}

type MarkAttendanceRequest struct {
	StudentID int    `json:"student_id" binding:"required"`
	Date      string `json:"date" binding:"required"`
	Status    string `json:"status" binding:"required,oneof=present absent late"`
	Notes     string `json:"notes"`
}

type BulkMarkAttendanceRequest struct {
	Date     string                  `json:"date" binding:"required"`
	BatchID  int                     `json:"batch_id"`
	CourseID int                     `json:"course_id"`
	Entries  []MarkAttendanceRequest `json:"entries" binding:"required,min=1"`
}

type AttendanceReport struct {
	Date           string  `json:"date"`
	Total          int     `json:"total"`
	Present        int     `json:"present"`
	Absent         int     `json:"absent"`
	Late           int     `json:"late"`
	AttendanceRate float64 `json:"attendance_rate"`
}

type Holiday struct {
	ID        int       `json:"id"`
	Date      string    `json:"date"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateHolidayRequest struct {
	Date   string `json:"date" binding:"required"`
	Reason string `json:"reason"`
}

type DeviceToken struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	Token     string    `json:"token"`
	Platform  string    `json:"platform"`
	CreatedAt time.Time `json:"created_at"`
}

type RegisterDeviceTokenRequest struct {
	Token    string `json:"token" binding:"required"`
	Platform string `json:"platform"`
}

// Doubt Resolution Tracker
type Doubt struct {
	ID              int        `json:"id"`
	StudentID       int        `json:"student_id"`
	StudentName     string     `json:"student_name"`
	QuestionText    string     `json:"question_text"`
	Subject         string     `json:"subject"`
	Chapter         string     `json:"chapter"`
	ImageURL        string     `json:"image_url"`
	Status          string     `json:"status"`
	Resolution      string     `json:"resolution"`
	ResolvedBy      int        `json:"resolved_by"`
	ResolvedByName  string     `json:"resolved_by_name"`
	ResolvedAt      *time.Time `json:"resolved_at"`
	ParentNotified  bool       `json:"parent_notified"`
	StudentNotified bool       `json:"student_notified"`
	CreatedAt       time.Time  `json:"created_at"`
}

type CreateDoubtRequest struct {
	QuestionText string `json:"question_text" binding:"required"`
	Subject      string `json:"subject"`
	Chapter      string `json:"chapter"`
	ImageURL     string `json:"image_url"`
}

type ResolveDoubtRequest struct {
	Resolution string `json:"resolution" binding:"required"`
}

// Smart Calendar
type CalendarEvent struct {
	ID               int       `json:"id"`
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	EventType        string    `json:"event_type"`
	Date             string    `json:"date"`
	EndDate          string    `json:"end_date"`
	CourseID         int       `json:"course_id"`
	CourseName       string    `json:"course_name"`
	BatchID          int       `json:"batch_id"`
	BatchName        string    `json:"batch_name"`
	Color            string    `json:"color"`
	IsAuto           bool      `json:"is_auto"`
	NotificationSent bool      `json:"notification_sent"`
	CreatedBy        int       `json:"created_by"`
	CreatedAt        time.Time `json:"created_at"`
}

type CreateCalendarEventRequest struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	EventType   string `json:"event_type" binding:"required"`
	Date        string `json:"date" binding:"required"`
	EndDate     string `json:"end_date"`
	CourseID    int    `json:"course_id"`
	BatchID     int    `json:"batch_id"`
	Color       string `json:"color"`
}

// Lesson Transparency
type Lesson struct {
	ID           int       `json:"id"`
	CourseID     int       `json:"course_id"`
	CourseName   string    `json:"course_name"`
	BatchID      int       `json:"batch_id"`
	BatchName    string    `json:"batch_name"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Subject      string    `json:"subject"`
	Chapter      string    `json:"chapter"`
	LessonDate   string    `json:"lesson_date"`
	TeacherNotes string    `json:"teacher_notes"`
	CreatedBy    int       `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
}

type CreateLessonRequest struct {
	CourseID     int    `json:"course_id" binding:"required"`
	BatchID      int    `json:"batch_id"`
	Title        string `json:"title" binding:"required"`
	Description  string `json:"description"`
	Subject      string `json:"subject"`
	Chapter      string `json:"chapter"`
	LessonDate   string `json:"lesson_date" binding:"required"`
	TeacherNotes string `json:"teacher_notes"`
}

// Digital Fee Payment
type Payment struct {
	ID             int        `json:"id"`
	UserID         int        `json:"user_id"`
	UserName       string     `json:"user_name"`
	UserMobile     string     `json:"user_mobile"`
	EnrollmentID   int        `json:"enrollment_id"`
	CourseID       int        `json:"course_id"`
	CourseName     string     `json:"course_name"`
	BatchID        int        `json:"batch_id"`
	BatchName      string     `json:"batch_name"`
	Amount         float64    `json:"amount"`
	Method         string     `json:"method"`
	TransactionID  string     `json:"transaction_id"`
	SenderNumber   string     `json:"sender_number"`
	ReceiverNumber string     `json:"receiver_number"`
	Status         string     `json:"status"`
	ReceiptNumber  string     `json:"receipt_number"`
	Month          string     `json:"month"`
	Year           int        `json:"year"`
	Notes          string     `json:"notes"`
	VerifiedBy     int        `json:"verified_by"`
	VerifiedByName string     `json:"verified_by_name"`
	VerifiedAt     *time.Time `json:"verified_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

type CreatePaymentRequest struct {
	UserID         int     `json:"user_id"`
	EnrollmentID   int     `json:"enrollment_id"`
	CourseID       int     `json:"course_id"`
	BatchID        int     `json:"batch_id"`
	Amount         float64 `json:"amount" binding:"required"`
	Method         string  `json:"method" binding:"required"`
	TransactionID  string  `json:"transaction_id"`
	SenderNumber   string  `json:"sender_number"`
	ReceiverNumber string  `json:"receiver_number"`
	Month          string  `json:"month"`
	Year           int     `json:"year"`
	Notes          string  `json:"notes"`
}

// Parenting Hub
type Article struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Category  string    `json:"category"`
	VideoURL  string    `json:"video_url"`
	ImageURL  string    `json:"image_url"`
	Published bool      `json:"is_published"`
	CreatedBy int       `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateArticleRequest struct {
	Title    string `json:"title" binding:"required"`
	Content  string `json:"content" binding:"required"`
	Category string `json:"category"`
	VideoURL string `json:"video_url"`
	ImageURL string `json:"image_url"`
}

// Notifications
type SendNotificationRequest struct {
	Title    string `json:"title" binding:"required"`
	Body     string `json:"body" binding:"required"`
	UserID   int    `json:"user_id"`
	BatchID  int    `json:"batch_id"`
	AllUsers bool   `json:"all_users"`
	LinkType string `json:"link_type"`
	LinkID   int    `json:"link_id"`
}

type Notification struct {
	ID       int       `json:"id"`
	Title    string    `json:"title"`
	Body     string    `json:"body"`
	Target   string    `json:"target"`
	TargetID int       `json:"target_id"`
	LinkType string    `json:"link_type"`
	LinkID   int       `json:"link_id"`
	SentBy   int       `json:"sent_by"`
	SentAt   time.Time `json:"sent_at"`
}

type NotificationWithRead struct {
	Notification
	ReadByMe bool `json:"read_by_me"`
}
