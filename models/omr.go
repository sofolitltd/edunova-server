package models

import "time"

type OMRQuestionPayload struct {
	QuestionNumber int `json:"question_number" binding:"required,min=1"`
	CorrectOption  int `json:"correct_option" binding:"required,min=1,max=4"`
}

// CreateOMRDesignRequest defines a reusable OMR sheet layout — the "OMR" in
// the admin UI's Create page. It has no answer key or roster; those belong
// to the tokens created from it.
type CreateOMRDesignRequest struct {
	Title         string `json:"title" binding:"required"`
	ClassLevel    string `json:"class_level"`
	Subject       string `json:"subject"`
	Columns       int    `json:"columns"`
	QuestionCount int    `json:"question_count" binding:"required,min=1"`
}

// UpdateOMRDesignRequest edits a design's layout. It doesn't touch any
// token already created from it — those keep their own copied layout.
type UpdateOMRDesignRequest struct {
	Title         string `json:"title" binding:"required"`
	ClassLevel    string `json:"class_level"`
	Subject       string `json:"subject"`
	Columns       int    `json:"columns"`
	QuestionCount int    `json:"question_count" binding:"required,min=1"`
}

type OMRDesign struct {
	ID            int       `json:"id"`
	Title         string    `json:"title"`
	ClassLevel    string    `json:"class_level"`
	Subject       string    `json:"subject"`
	QuestionCount int       `json:"question_count"`
	Columns       int       `json:"columns"`
	TokenCount    int       `json:"token_count"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CreateOMRTokenRequest creates one exam instance ("token") from an
// already-created OMRDesign. Its answer key and roster are set afterward,
// on the token's own detail page.
type CreateOMRTokenRequest struct {
	Title       string `json:"title" binding:"required"`
	OMRDesignID int    `json:"omr_design_id" binding:"required"`
}

// UpdateOMRAnswerKeyRequest sets (or replaces) a token's answer key. Every
// question number from 1 to the token's question count must be included.
type UpdateOMRAnswerKeyRequest struct {
	Questions []OMRQuestionPayload `json:"questions" binding:"required,min=1"`
}

// UpdateOMRTokenRequest renames a token. Its layout (question count/columns)
// came from its design at creation time and isn't editable here — changing
// it would orphan the answer key and any sheets already scored against it.
type UpdateOMRTokenRequest struct {
	Title string `json:"title" binding:"required"`
}

type OMRExam struct {
	ID            int       `json:"id"`
	Title         string    `json:"title"`
	ClassLevel    string    `json:"class_level"`
	Subject       string    `json:"subject"`
	QuestionCount int       `json:"question_count"`
	Columns       int       `json:"columns"`
	ExamCode      string    `json:"exam_code"`
	OMRDesignID   *int      `json:"omr_design_id"`
	AnswerKeySet  bool      `json:"answer_key_set"`
	StudentCount  int       `json:"student_count"`
	SheetCount    int       `json:"sheet_count"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type OMRQuestion struct {
	ID             int  `json:"id"`
	OMRExamID      int  `json:"omr_exam_id"`
	QuestionNumber int  `json:"question_number"`
	CorrectOption  *int `json:"correct_option"`
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
	ID        int `json:"id"`
	OMRExamID int `json:"omr_exam_id"`
	// AnnotatedPreview is a data: URL of the marked-up sheet, generated in
	// memory and set only on the upload response — nothing about the sheet's
	// image is stored, so this is empty on every later read (list/get).
	AnnotatedPreview   string               `json:"annotated_preview,omitempty"`
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
