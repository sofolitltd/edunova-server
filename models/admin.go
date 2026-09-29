package models

import (
	"encoding/json"
	"time"
)

type AdminUser struct {
	ID           int       `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	FullName     string    `json:"full_name"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type AdminLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type AdminAuthResponse struct {
	Token string    `json:"token"`
	Admin AdminUser `json:"admin"`
}

type UpdateAdminProfileRequest struct {
	FullName string `json:"full_name" binding:"required,min=2"`
	Email    string `json:"email" binding:"required,email"`
}

type ChangeAdminPasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

type CreateAdminRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	FullName string `json:"full_name" binding:"required"`
	Role     string `json:"role" binding:"required"`
}

type Course struct {
	ID            int             `json:"id"`
	Title         string          `json:"title"`
	TitleBn       string          `json:"title_bn"`
	Description   string          `json:"description"`
	Subject       string          `json:"subject"`
	Teacher       string          `json:"teacher"`
	Instructors   string          `json:"instructors"`
	ClassLevel    string          `json:"class_level"`
	Type          string          `json:"type"`
	Schedule      string          `json:"schedule"`
	Color         string          `json:"color"`
	Gradient      string          `json:"gradient"`
	Price         int             `json:"price"`
	OldPrice      int             `json:"old_price"`
	Duration      string          `json:"duration"`
	Badge         string          `json:"badge"`
	StudentsCount int             `json:"students_count"`
	ClassesCount  int             `json:"classes_count"`
	ExamsCount    int             `json:"exams_count"`
	Rating        float64         `json:"rating"`
	ReviewsCount  int             `json:"reviews_count"`
	Curriculum    json.RawMessage `json:"curriculum"`
	Features      json.RawMessage `json:"features"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type CreateCourseRequest struct {
	Title         string          `json:"title" binding:"required,min=3"`
	TitleBn       string          `json:"title_bn"`
	Description   string          `json:"description"`
	Subject       string          `json:"subject" binding:"required"`
	Teacher       string          `json:"teacher" binding:"required"`
	Instructors   string          `json:"instructors"`
	ClassLevel    string          `json:"class_level"`
	Type          string          `json:"type"`
	Schedule      string          `json:"schedule"`
	Color         string          `json:"color"`
	Gradient      string          `json:"gradient"`
	Price         int             `json:"price"`
	OldPrice      int             `json:"old_price"`
	Duration      string          `json:"duration"`
	Badge         string          `json:"badge"`
	StudentsCount int             `json:"students_count"`
	ClassesCount  int             `json:"classes_count"`
	ExamsCount    int             `json:"exams_count"`
	Rating        float64         `json:"rating"`
	ReviewsCount  int             `json:"reviews_count"`
	Curriculum    json.RawMessage `json:"curriculum"`
	Features      json.RawMessage `json:"features"`
}

type Exam struct {
	ID             int       `json:"id"`
	Title          string    `json:"title"`
	CourseID       int       `json:"course_id"`
	CourseName     string    `json:"course_name"`
	BatchID        int       `json:"batch_id"`
	BatchName      string    `json:"batch_name"`
	Date           string    `json:"date"`
	Time           string    `json:"time"`
	Duration       string    `json:"duration"`
	TotalQuestions int       `json:"total_questions"`
	TotalMarks     int       `json:"total_marks"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type CreateExamRequest struct {
	Title      string                `json:"title" binding:"required"`
	CourseID   int                   `json:"course_id"`
	BatchID    int                   `json:"batch_id"`
	Date       string                `json:"date" binding:"required"`
	Time       string                `json:"time" binding:"required"`
	Duration   string                `json:"duration" binding:"required"`
	TotalMarks int                   `json:"total_marks"`
	Questions  []ExamQuestionPayload `json:"questions"`
}

type ExamQuestionPayload struct {
	QuestionText  string `json:"question_text" binding:"required"`
	OptionA       string `json:"option_a" binding:"required"`
	OptionB       string `json:"option_b" binding:"required"`
	OptionC       string `json:"option_c" binding:"required"`
	OptionD       string `json:"option_d" binding:"required"`
	CorrectOption int    `json:"correct_option" binding:"required,min=1,max=4"`
}

type ExamQuestion struct {
	ID            int       `json:"id"`
	ExamID        int       `json:"exam_id"`
	QuestionText  string    `json:"question_text"`
	OptionA       string    `json:"option_a"`
	OptionB       string    `json:"option_b"`
	OptionC       string    `json:"option_c"`
	OptionD       string    `json:"option_d"`
	CorrectOption int       `json:"correct_option"`
	CreatedAt     time.Time `json:"created_at"`
}

type ExamResult struct {
	ID             int       `json:"id"`
	ExamID         int       `json:"exam_id"`
	ExamTitle      string    `json:"exam_title"`
	UserID         int       `json:"user_id"`
	UserName       string    `json:"user_name"`
	Score          int       `json:"score"`
	TotalQuestions int       `json:"total_questions"`
	CompletedAt    time.Time `json:"completed_at"`
}

type DashboardStats struct {
	TotalUsers          int                 `json:"total_users"`
	VerifiedUsers       int                 `json:"verified_users"`
	TotalCourses        int                 `json:"total_courses"`
	TotalExams          int                 `json:"total_exams"`
	WeeklyRegistrations []DailyRegistration `json:"weekly_registrations"`
}

type DailyRegistration struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

type PaginatedUsers struct {
	Users      []User `json:"users"`
	Total      int    `json:"total"`
	Page       int    `json:"page"`
	PerPage    int    `json:"per_page"`
	TotalPages int    `json:"total_pages"`
}

type Enrollment struct {
	ID             int       `json:"id"`
	CourseID       int       `json:"course_id"`
	CourseName     string    `json:"course_name"`
	CourseType     string    `json:"course_type"`
	FullName       string    `json:"full_name"`
	Mobile         string    `json:"mobile"`
	StudentID      string    `json:"student_id"`
	UserID         *int      `json:"user_id"`
	PaymentMethod  string    `json:"payment_method"`
	MobileBanking  string    `json:"mobile_banking"`
	Amount         int       `json:"amount"`
	SentFrom       string    `json:"sent_from"`
	SentTo         string    `json:"sent_to"`
	ReferralSource string    `json:"referral_source"`
	Status         string    `json:"status"`
	EnrolledBy     string    `json:"enrolled_by"`
	BatchID        *int      `json:"batch_id"`
	BatchName      string    `json:"batch_name"`
	BatchSchedule  string    `json:"batch_schedule"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type CreateEnrollmentRequest struct {
	CourseID       int    `json:"course_id" binding:"required"`
	FullName       string `json:"full_name" binding:"required"`
	Mobile         string `json:"mobile" binding:"required"`
	PaymentMethod  string `json:"payment_method"`
	MobileBanking  string `json:"mobile_banking"`
	Amount         int    `json:"amount"`
	SentFrom       string `json:"sent_from"`
	SentTo         string `json:"sent_to"`
	ReferralSource string `json:"referral_source"`
}

type DirectEnrollRequest struct {
	UserID             int    `json:"user_id"`
	Mobile             string `json:"mobile" binding:"required"`
	CourseID           int    `json:"course_id"`
	BatchID            *int   `json:"batch_id"`
	Amount             int    `json:"amount"`
	FullName           string `json:"full_name"`
	StudentID          string `json:"student_id"`
	Gender             string `json:"gender"`
	StudentClass       string `json:"student_class"`
	School             string `json:"school"`
	SchoolShift        string `json:"shift"`
	FatherName         string `json:"father_name"`
	FatherMobile       string `json:"father_mobile"`
	MotherName         string `json:"mother_name"`
	MotherMobile       string `json:"mother_mobile"`
	NotificationMobile string `json:"notification_mobile"`
	Address            string `json:"address"`
	PaymentMethod      string `json:"payment_method"`
	Reference          string `json:"reference"`
}

type UpdateEnrollmentStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=pending approved rejected"`
}

type PaginatedEnrollments struct {
	Enrollments []Enrollment `json:"enrollments"`
	Total       int          `json:"total"`
	Page        int          `json:"page"`
	PerPage     int          `json:"per_page"`
	TotalPages  int          `json:"total_pages"`
}

// ==================== Question Bank Models ====================

type Class struct {
	ID         int       `json:"id"`
	Name       string    `json:"name"`
	NameBn     string    `json:"name_bn"`
	OrderIndex int       `json:"order_index"`
	Code       string    `json:"code"`
	CreatedAt  time.Time `json:"created_at"`
}

type CreateClassRequest struct {
	Name       string `json:"name" binding:"required"`
	NameBn     string `json:"name_bn"`
	OrderIndex int    `json:"order_index"`
	Code       string `json:"code"`
}

type Subject struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	NameBn    string    `json:"name_bn"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateSubjectRequest struct {
	Name   string `json:"name" binding:"required"`
	NameBn string `json:"name_bn"`
}

type Book struct {
	ID          int       `json:"id"`
	SubjectID   int       `json:"subject_id"`
	ClassName   string    `json:"class_name"`
	SubjectName string    `json:"subject_name"`
	Name        string    `json:"name"`
	NameBn      string    `json:"name_bn"`
	Publisher   string    `json:"publisher"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreateBookRequest struct {
	SubjectID int    `json:"subject_id" binding:"required"`
	ClassID   int    `json:"class_id" binding:"required"`
	Name      string `json:"name" binding:"required"`
	NameBn    string `json:"name_bn"`
	Publisher string `json:"publisher"`
}

type Chapter struct {
	ID         int       `json:"id"`
	BookID     int       `json:"book_id"`
	BookName   string    `json:"book_name"`
	Name       string    `json:"name"`
	NameBn     string    `json:"name_bn"`
	OrderIndex int       `json:"order_index"`
	CreatedAt  time.Time `json:"created_at"`
}

type CreateChapterRequest struct {
	BookID     int    `json:"book_id" binding:"required"`
	Name       string `json:"name" binding:"required"`
	NameBn     string `json:"name_bn"`
	OrderIndex int    `json:"order_index"`
}

type Topic struct {
	ID          int       `json:"id"`
	ChapterID   int       `json:"chapter_id"`
	ChapterName string    `json:"chapter_name"`
	Name        string    `json:"name"`
	NameBn      string    `json:"name_bn"`
	OrderIndex  int       `json:"order_index"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreateTopicRequest struct {
	ChapterID  int    `json:"chapter_id" binding:"required"`
	Name       string `json:"name" binding:"required"`
	NameBn     string `json:"name_bn"`
	OrderIndex int    `json:"order_index"`
}

type Question struct {
	ID           int        `json:"id"`
	ClassID      *int       `json:"class_id"`
	ClassName    string     `json:"class_name"`
	SubjectID    *int       `json:"subject_id"`
	SubjectName  string     `json:"subject_name"`
	BookID       *int       `json:"book_id"`
	BookName     string     `json:"book_name"`
	ChapterID    *int       `json:"chapter_id"`
	ChapterName  string     `json:"chapter_name"`
	TopicID      *int       `json:"topic_id"`
	TopicName    string     `json:"topic_name"`
	QuestionType string     `json:"question_type"`
	QuestionText string     `json:"question_text"`
	Options      *string    `json:"options"`
	Answer       string     `json:"answer"`
	Explanation  string     `json:"explanation"`
	Marks        int        `json:"marks"`
	Difficulty   string     `json:"difficulty"`
	Tags         []string   `json:"tags"`
	Source       string     `json:"source"`
	SourcePage   *int       `json:"source_page"`
	Language     string     `json:"language"`
	Status       string     `json:"status"`
	CreatedBy    *int       `json:"created_by"`
	ReviewedBy   *int       `json:"reviewed_by"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	Version      int        `json:"version"`
	UsageCount   int        `json:"usage_count"`
	LastUsedAt   *time.Time `json:"last_used_at"`
}

type CreateQuestionRequest struct {
	ClassID      *int     `json:"class_id"`
	SubjectID    *int     `json:"subject_id"`
	BookID       *int     `json:"book_id"`
	ChapterID    *int     `json:"chapter_id"`
	TopicID      *int     `json:"topic_id"`
	QuestionType string   `json:"question_type" binding:"required"`
	QuestionText string   `json:"question_text" binding:"required"`
	Options      *string  `json:"options"`
	Answer       string   `json:"answer" binding:"required"`
	Explanation  string   `json:"explanation"`
	Marks        int      `json:"marks"`
	Difficulty   string   `json:"difficulty"`
	Tags         []string `json:"tags"`
	Source       string   `json:"source"`
	SourcePage   *int     `json:"source_page"`
	Language     string   `json:"language"`
}

type UpdateQuestionStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=draft published archived"`
}

type PaginatedQuestions struct {
	Questions  []Question `json:"questions"`
	Total      int        `json:"total"`
	Page       int        `json:"page"`
	PerPage    int        `json:"per_page"`
	TotalPages int        `json:"total_pages"`
}

type QuestionBankStats struct {
	Total        int            `json:"total"`
	ByStatus     map[string]int `json:"by_status"`
	ByType       map[string]int `json:"by_type"`
	ByDifficulty map[string]int `json:"by_difficulty"`
	ByClass      []ClassStats   `json:"by_class"`
	ByChapter    []ChapterStats `json:"by_chapter"`
}

type ClassStats struct {
	ClassID   int            `json:"class_id"`
	ClassName string         `json:"class_name"`
	Total     int            `json:"total"`
	ByType    map[string]int `json:"by_type"`
}

type ChapterStats struct {
	ChapterID   int            `json:"chapter_id"`
	ChapterName string         `json:"chapter_name"`
	ClassName   string         `json:"class_name"`
	SubjectName string         `json:"subject_name"`
	Total       int            `json:"total"`
	ByType      map[string]int `json:"by_type"`
}

type BulkQuestion struct {
	Class        string `json:"class"`
	Subject      string `json:"subject"`
	Book         string `json:"book"`
	Chapter      string `json:"chapter"`
	Topic        string `json:"topic"`
	QuestionType string `json:"question_type"`
	Question     string `json:"question"`
	OptionA      string `json:"option_a"`
	OptionB      string `json:"option_b"`
	OptionC      string `json:"option_c"`
	OptionD      string `json:"option_d"`
	Answer       string `json:"answer"`
	Explanation  string `json:"explanation"`
	Marks        int    `json:"marks"`
	Difficulty   string `json:"difficulty"`
	Tags         string `json:"tags"`
}

type BulkUploadRequest struct {
	Questions []BulkQuestion `json:"questions"`
}

type QuestionImport struct {
	ID           int        `json:"id"`
	Filename     string     `json:"filename"`
	TotalRows    int        `json:"total_rows"`
	Imported     int        `json:"imported"`
	Duplicates   int        `json:"duplicates"`
	Errors       int        `json:"errors"`
	ErrorDetails *string    `json:"error_details"`
	Status       string     `json:"status"`
	CreatedBy    *int       `json:"created_by"`
	CreatedAt    time.Time  `json:"created_at"`
	CompletedAt  *time.Time `json:"completed_at"`
}

type BulkUploadResult struct {
	ImportID   int              `json:"import_id"`
	TotalRows  int              `json:"total_rows"`
	Imported   int              `json:"imported"`
	Duplicates int              `json:"duplicates"`
	Errors     int              `json:"errors"`
	ErrorRows  []ImportErrorRow `json:"error_rows"`
}

type ImportErrorRow struct {
	Row    int      `json:"row"`
	Errors []string `json:"errors"`
}

type DuplicateCheckResult struct {
	IsDuplicate bool   `json:"is_duplicate"`
	Reason      string `json:"reason"`
	SimilarID   *int   `json:"similar_id"`
}

type BulkHierarchyResult struct {
	Created int      `json:"created"`
	Skipped int      `json:"skipped"`
	Errors  []string `json:"errors"`
}

type Expense struct {
	ID          int       `json:"id"`
	Category    string    `json:"category"`
	Description string    `json:"description"`
	Amount      int       `json:"amount"`
	Date        string    `json:"date"`
	Notes       string    `json:"notes"`
	CreatedBy   int       `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreateExpenseRequest struct {
	Category    string `json:"category" binding:"required"`
	Description string `json:"description" binding:"required"`
	Amount      int    `json:"amount" binding:"required"`
	Date        string `json:"date"`
	Notes       string `json:"notes"`
}

type MonthlyFinance struct {
	Month    string `json:"month"`
	Revenue  int    `json:"revenue"`
	Expenses int    `json:"expenses"`
}

type FinanceStats struct {
	TotalRevenue      int              `json:"total_revenue"`
	TotalExpenses     int              `json:"total_expenses"`
	NetProfit         int              `json:"net_profit"`
	PendingPayments   int              `json:"pending_payments"`
	ApprovedPayments  int              `json:"approved_payments"`
	MonthlyData       []MonthlyFinance `json:"monthly_data"`
	RecentEnrollments []Enrollment     `json:"recent_enrollments"`
	RecentExpenses    []Expense        `json:"recent_expenses"`
}

type BatchMonthlyRevenue struct {
	Month   string `json:"month"`
	Revenue int    `json:"revenue"`
}

type BatchFinanceStats struct {
	TotalRevenue    int                   `json:"total_revenue"`
	ApprovedCount   int                   `json:"approved_count"`
	PendingPayments int                   `json:"pending_payments"`
	PendingCount    int                   `json:"pending_count"`
	MonthlyData     []BatchMonthlyRevenue `json:"monthly_data"`
	RecentPayments  []Enrollment          `json:"recent_payments"`
}

type BatchAttendanceDay struct {
	Date    string `json:"date"`
	Present int    `json:"present"`
	Absent  int    `json:"absent"`
	Late    int    `json:"late"`
}

type BatchStudentAttendance struct {
	StudentID int     `json:"student_id"`
	FullName  string  `json:"full_name"`
	Present   int     `json:"present"`
	Absent    int     `json:"absent"`
	Late      int     `json:"late"`
	Rate      float64 `json:"rate"`
}

type BatchMonthlyAttendance struct {
	Month        string                   `json:"month"`
	TotalClasses int                      `json:"total_classes"`
	OverallRate  float64                  `json:"overall_rate"`
	Days         []BatchAttendanceDay     `json:"days"`
	Students     []BatchStudentAttendance `json:"students"`
}

type Batch struct {
	ID           int       `json:"id"`
	ClassLevel   string    `json:"class_level"`
	CourseID     *int      `json:"course_id"`
	CourseName   string    `json:"course_name"`
	Name         string    `json:"name"`
	Days         []string  `json:"days"`
	StartTime    string    `json:"start_time"`
	EndTime      string    `json:"end_time"`
	Schedule     string    `json:"schedule"`
	MaxStudents  int       `json:"max_students"`
	Status       string    `json:"status"`
	AdmissionFee int       `json:"admission_fee"`
	NoteFee      int       `json:"note_fee"`
	MonthlyFee   int       `json:"monthly_fee"`
	Shift        string    `json:"shift"`
	Type         string    `json:"type"`
	Year         int       `json:"year"`
	Section      string    `json:"section"`
	Code         string    `json:"code"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type CreateBatchRequest struct {
	ClassLevel   string   `json:"class_level" binding:"required"`
	CourseID     *int     `json:"course_id"`
	Name         string   `json:"name" binding:"required"`
	Days         []string `json:"days"`
	StartTime    string   `json:"start_time"`
	EndTime      string   `json:"end_time"`
	MaxStudents  int      `json:"max_students"`
	Status       string   `json:"status"`
	AdmissionFee int      `json:"admission_fee"`
	NoteFee      int      `json:"note_fee"`
	MonthlyFee   int      `json:"monthly_fee"`
	Shift        string   `json:"shift"`
	Type         string   `json:"type"`
	Year         int      `json:"year"`
	Section      string   `json:"section" binding:"required"`
	Code         string   `json:"code" binding:"required"`
}

// BatchSubject is a subject taught within a batch, with its own weekly
// schedule and assigned teacher (a batch can teach several subjects at
// different day/time slots, each with its own teacher).
type BatchSubject struct {
	ID          int      `json:"id"`
	SubjectID   int      `json:"subject_id"`
	SubjectName string   `json:"subject_name"`
	TeacherID   *int     `json:"teacher_id"`
	TeacherName string   `json:"teacher_name"`
	Days        []string `json:"days"`
	StartTime   string   `json:"start_time"`
	EndTime     string   `json:"end_time"`
}

type AssignBatchSubjectRequest struct {
	SubjectID int      `json:"subject_id" binding:"required"`
	TeacherID *int     `json:"teacher_id"`
	Days      []string `json:"days"`
	StartTime string   `json:"start_time"`
	EndTime   string   `json:"end_time"`
}

type Note struct {
	ID          int       `json:"id"`
	ClassLevel  string    `json:"class_level"`
	Subject     string    `json:"subject"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	Tags        []string  `json:"tags"`
	IsPublished bool      `json:"is_published"`
	CreatedBy   *int      `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateNoteRequest struct {
	ClassLevel  string   `json:"class_level" binding:"required"`
	Subject     string   `json:"subject" binding:"required"`
	Title       string   `json:"title" binding:"required"`
	Content     string   `json:"content" binding:"required"`
	Tags        []string `json:"tags"`
	IsPublished *bool    `json:"is_published"`
}

type DailyContent struct {
	ID          int       `json:"id"`
	ContentType string    `json:"content_type"`
	ClassLevel  string    `json:"class_level"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	Answer      string    `json:"answer"`
	Language    string    `json:"language"`
	IsPublished bool      `json:"is_published"`
	CreatedBy   *int      `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreateDailyContentRequest struct {
	ContentType string `json:"content_type" binding:"required"`
	ClassLevel  string `json:"class_level" binding:"required"`
	Title       string `json:"title" binding:"required"`
	Body        string `json:"body" binding:"required"`
	Answer      string `json:"answer"`
	Language    string `json:"language"`
}

type DailyContentDelivery struct {
	ID           int        `json:"id"`
	ContentID    int        `json:"content_id"`
	ContentType  string     `json:"content_type"`
	Title        string     `json:"title"`
	Body         string     `json:"body"`
	Answer       string     `json:"answer"`
	DeliveredAt  time.Time  `json:"delivered_at"`
	ViewedAt     *time.Time `json:"viewed_at"`
	NextReviewAt *time.Time `json:"next_review_at"`
	ReviewCount  int        `json:"review_count"`
}

type StudentTransition struct {
	ID          int             `json:"id"`
	UserID      int             `json:"user_id"`
	UserName    string          `json:"user_name"`
	FromClass   string          `json:"from_class"`
	ToClass     string          `json:"to_class"`
	GPA         float64         `json:"gpa"`
	Subjects    json.RawMessage `json:"subjects"`
	ResultNotes string          `json:"result_notes"`
	SubmittedAt time.Time       `json:"submitted_at"`
}

type CreateTransitionRequest struct {
	FromClass   string          `json:"from_class" binding:"required"`
	ToClass     string          `json:"to_class" binding:"required"`
	GPA         float64         `json:"gpa"`
	Subjects    json.RawMessage `json:"subjects"`
	ResultNotes string          `json:"result_notes"`
}

type StudentFeedback struct {
	ID           int             `json:"id"`
	TransitionID int             `json:"transition_id"`
	UserID       int             `json:"user_id"`
	UserName     string          `json:"user_name"`
	FeedbackType string          `json:"feedback_type"`
	Title        string          `json:"title"`
	Message      string          `json:"message"`
	Guidelines   json.RawMessage `json:"guidelines"`
	CreatedAt    time.Time       `json:"created_at"`
}

type AdminToggleLiveRequest struct {
	IsLive bool `json:"is_live"`
}

// ==================== Results / Progress ====================

type StudentResult struct {
	ID            int       `json:"id"`
	UserID        int       `json:"user_id"`
	StudentName   string    `json:"student_name"`
	StudentClass  string    `json:"student_class"`
	Subject       string    `json:"subject"`
	ExamName      string    `json:"exam_name"`
	ExamDate      string    `json:"exam_date"`
	MarksObtained float64   `json:"marks_obtained"`
	MarksTotal    float64   `json:"marks_total"`
	Percentage    float64   `json:"percentage"`
	Remarks       string    `json:"remarks"`
	CreatedAt     time.Time `json:"created_at"`
}

type CreateResultRequest struct {
	UserID        int     `json:"user_id" binding:"required"`
	Subject       string  `json:"subject" binding:"required"`
	ExamName      string  `json:"exam_name" binding:"required"`
	ExamDate      string  `json:"exam_date" binding:"required"`
	MarksObtained float64 `json:"marks_obtained" binding:"required,min=0"`
	MarksTotal    float64 `json:"marks_total" binding:"required,min=1"`
	Remarks       string  `json:"remarks"`
}

type ResultSubjectSummary struct {
	Subject        string  `json:"subject"`
	AveragePercent float64 `json:"average_percent"`
	ExamCount      int     `json:"exam_count"`
}

type ResultSummary struct {
	OverallPercent   float64                `json:"overall_percent"`
	TotalExams       int                    `json:"total_exams"`
	BySubject        []ResultSubjectSummary `json:"by_subject"`
	ImprovementAreas []ResultSubjectSummary `json:"improvement_areas"`
	Recent           []StudentResult        `json:"recent"`
}

// ==================== Interactive Practice ====================

type VocabularyWord struct {
	ID              int       `json:"id"`
	ClassLevel      string    `json:"class_level"`
	Word            string    `json:"word"`
	Meaning         string    `json:"meaning"`
	MeaningBn       string    `json:"meaning_bn"`
	ExampleSentence string    `json:"example_sentence"`
	Pronunciation   string    `json:"pronunciation"`
	CreatedAt       time.Time `json:"created_at"`
}

type CreateVocabularyRequest struct {
	ClassLevel      string `json:"class_level" binding:"required,oneof=3 4 5 6 7 8"`
	Word            string `json:"word" binding:"required"`
	Meaning         string `json:"meaning" binding:"required"`
	MeaningBn       string `json:"meaning_bn"`
	ExampleSentence string `json:"example_sentence"`
	Pronunciation   string `json:"pronunciation"`
}

type SentenceExercise struct {
	ID           int       `json:"id"`
	ClassLevel   string    `json:"class_level"`
	Prompt       string    `json:"prompt"`
	SampleAnswer string    `json:"sample_answer"`
	CreatedAt    time.Time `json:"created_at"`
}

type CreateSentenceExerciseRequest struct {
	ClassLevel   string `json:"class_level" binding:"required,oneof=3 4 5 6 7 8"`
	Prompt       string `json:"prompt" binding:"required"`
	SampleAnswer string `json:"sample_answer"`
}

// QuizQuestion is generated on the fly from vocabulary_words — one word is
// the correct answer, three distractor meanings come from other words in
// the same class.
type QuizQuestion struct {
	WordID  int      `json:"word_id"`
	Word    string   `json:"word"`
	Options []string `json:"options"`
}

type QuizAttemptRequest struct {
	WordID        int    `json:"word_id" binding:"required"`
	SelectedIndex int    `json:"selected_index"`
	SelectedText  string `json:"selected_text" binding:"required"`
}

type SentenceAttemptRequest struct {
	ExerciseID int    `json:"exercise_id" binding:"required"`
	Answer     string `json:"answer" binding:"required"`
}

type FlashcardReviewRequest struct {
	WordID int `json:"word_id" binding:"required"`
}

type PracticeStats struct {
	TotalPoints    int `json:"total_points"`
	Level          int `json:"level"`
	PointsToNext   int `json:"points_to_next_level"`
	CurrentStreak  int `json:"current_streak"`
	LongestStreak  int `json:"longest_streak"`
	QuizAttempts   int `json:"quiz_attempts"`
	QuizCorrect    int `json:"quiz_correct"`
	FlashcardsSeen int `json:"flashcards_seen"`
	SentencesDone  int `json:"sentences_done"`
}
