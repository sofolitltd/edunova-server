package models

import "time"

type OMRQuestionPayload struct {
	QuestionNumber int `json:"question_number" binding:"required,min=1"`
	CorrectOption  int `json:"correct_option" binding:"required,min=1,max=4"`
}

type CreateOMRExamRequest struct {
	Title      string               `json:"title" binding:"required"`
	ClassLevel string               `json:"class_level"`
	Subject    string               `json:"subject"`
	Columns    int                  `json:"columns"`
	Questions  []OMRQuestionPayload `json:"questions" binding:"required,min=1"`
}

type OMRExam struct {
	ID            int       `json:"id"`
	Title         string    `json:"title"`
	ClassLevel    string    `json:"class_level"`
	Subject       string    `json:"subject"`
	QuestionCount int       `json:"question_count"`
	Columns       int       `json:"columns"`
	ExamCode      string    `json:"exam_code"`
	StudentCount  int       `json:"student_count"`
	SheetCount    int       `json:"sheet_count"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type OMRQuestion struct {
	ID             int `json:"id"`
	OMRExamID      int `json:"omr_exam_id"`
	QuestionNumber int `json:"question_number"`
	CorrectOption  int `json:"correct_option"`
}

type OMRStudentPayload struct {
	RollNumber string `json:"roll_number" binding:"required"`
	Name       string `json:"name"`
}

type AddOMRStudentsRequest struct {
	Students []OMRStudentPayload `json:"students" binding:"required,min=1"`
}

type OMRStudent struct {
	ID           int    `json:"id"`
	OMRExamID    int    `json:"omr_exam_id"`
	RollNumber   string `json:"roll_number"`
	Name         string `json:"name"`
	EnrollmentID *int   `json:"enrollment_id"`
}

type ImportOMRRosterRequest struct {
	BatchID int `json:"batch_id" binding:"required"`
}

type OMRQuestionOutcome struct {
	QuestionNumber int  `json:"question_number"`
	SelectedOption int  `json:"selected_option"`
	CorrectOption  int  `json:"correct_option"`
	Correct        bool `json:"correct"`
	Ambiguous      bool `json:"ambiguous"`
}

type OMRSheet struct {
	ID                 int                  `json:"id"`
	OMRExamID          int                  `json:"omr_exam_id"`
	ImagePath          string               `json:"image_path"`
	DetectedRollNumber string               `json:"detected_roll_number"`
	MatchedStudentID   *int                 `json:"matched_student_id"`
	MatchedStudentName string               `json:"matched_student_name,omitempty"`
	Status             string               `json:"status"` // scored | needs_review | unreadable
	Score              int                  `json:"score"`
	TotalQuestions     int                  `json:"total_questions"`
	DetectedExamCode   string               `json:"detected_exam_code,omitempty"`
	ExamCodeMismatch   bool                 `json:"exam_code_mismatch,omitempty"`
	RollAmbiguous      bool                 `json:"roll_ambiguous,omitempty"`
	Questions          []OMRQuestionOutcome `json:"questions,omitempty"`
	StudentResultID    *int                 `json:"student_result_id"`
	GradebookSynced    bool                 `json:"gradebook_synced"`
	CreatedAt          time.Time            `json:"created_at"`
}

// OMRAnswerCorrection overrides the detected selected option for one
// question on an already-scored sheet.
type OMRAnswerCorrection struct {
	QuestionNumber  int `json:"question_number" binding:"required,min=1"`
	CorrectedOption int `json:"corrected_option" binding:"required,min=1,max=4"`
}

// UpdateOMRSheetRequest lets an admin fix misread bubbles (or the
// roll-to-student match) on a sheet that came back needs_review.
type UpdateOMRSheetRequest struct {
	Corrections      []OMRAnswerCorrection `json:"corrections"`
	MatchedStudentID *int                  `json:"matched_student_id"`
}
