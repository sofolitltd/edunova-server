package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"edunova-server/database"
	"edunova-server/models"
	"edunova-server/services/omr"
)

// annotatedPreviewDataURL JPEG-encodes img entirely in memory and returns it
// as a data: URL — nothing is written to disk. The annotated overlay is a
// one-time review aid for right after evaluating, not stored data, so it
// only ever travels in the upload response and is gone once that response
// is sent.
func annotatedPreviewDataURL(img image.Image) string {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return ""
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

// headerOnlyTemplate builds a template just to know where the fiducial
// markers, roll-number grid and exam-code grid are — that part of the
// layout is the same for every exam regardless of question count/columns,
// so it's enough to identify a sheet before its real template is known.
func headerOnlyTemplate() omr.Template {
	return omr.BuildTemplate(omr.MinQuestions, 1)
}

// syncOMRSheetGradebook posts (or, if existingResultID is set, updates) a
// student_results row for a scored sheet, so the scoring shows up in the
// same Results screen the student app already reads. It's a no-op — not an
// error — when the matched roster row has no linked enrollment, or that
// enrollment has no registered app login yet.
func syncOMRSheetGradebook(ctx context.Context, adminID int, examID int, matchedStudentID *int, score, totalQuestions int, existingResultID *int) *int {
	if matchedStudentID == nil {
		return existingResultID
	}
	var userID *int
	if err := database.DB.QueryRow(ctx,
		`SELECT e.user_id FROM omr_students s JOIN enrollments e ON e.id = s.enrollment_id WHERE s.id = $1`, *matchedStudentID,
	).Scan(&userID); err != nil || userID == nil {
		return existingResultID
	}

	if existingResultID != nil {
		if _, err := database.DB.Exec(ctx,
			`UPDATE student_results SET marks_obtained = $1, marks_total = $2, updated_at = NOW() WHERE id = $3`,
			float64(score), float64(totalQuestions), *existingResultID,
		); err != nil {
			return existingResultID
		}
		return existingResultID
	}

	var examTitle, subject string
	if err := database.DB.QueryRow(ctx, `SELECT title, subject FROM omr_exams WHERE id = $1`, examID).Scan(&examTitle, &subject); err != nil {
		return nil
	}
	if strings.TrimSpace(subject) == "" {
		subject = "OMR"
	}

	var createdBy interface{}
	if adminID != 0 {
		createdBy = adminID
	}

	var resultID int
	if err := database.DB.QueryRow(ctx,
		`INSERT INTO student_results (user_id, subject, exam_name, exam_date, marks_obtained, marks_total, remarks, created_by)
		 VALUES ($1, $2, $3, CURRENT_DATE, $4, $5, '', $6) RETURNING id`,
		*userID, subject, examTitle, float64(score), float64(totalQuestions), createdBy,
	).Scan(&resultID); err != nil {
		return nil
	}
	return &resultID
}

// AdminUploadOMRSheet accepts a photo of a filled bubble sheet, decodes it,
// and scores it against the exam identified by the sheet's own printed exam
// code (no need to pre-select an exam) or an explicit `exam_id` form field
// as a fallback if the code can't be read. The photo itself is never
// written to disk or the database — only the scored result is persisted;
// an annotated preview of the marks is generated in memory and returned
// just for this one response.
func AdminUploadOMRSheet(c *gin.Context) {
	file, _, err := c.Request.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "an 'image' file is required"})
		return
	}
	defer file.Close()

	img, err := omr.DecodeImage(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "could not read the uploaded image — use a clear JPEG or PNG photo of the full sheet"})
		return
	}

	ctx := c.Request.Context()

	headerResult, err := omr.Detect(img, headerOnlyTemplate())
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, models.ErrorResponse{Error: err.Error()})
		return
	}

	examID := 0
	if forced := strings.TrimSpace(c.PostForm("exam_id")); forced != "" {
		examID, _ = strconv.Atoi(forced)
	}

	var questionCount, columns int
	var examTitle, examCode string
	var lookupErr error
	if examID > 0 {
		lookupErr = database.DB.QueryRow(ctx,
			`SELECT question_count, columns, title, exam_code FROM omr_exams WHERE id = $1`, examID,
		).Scan(&questionCount, &columns, &examTitle, &examCode)
	} else if !headerResult.ExamCodeAmbiguous {
		lookupErr = database.DB.QueryRow(ctx,
			`SELECT id, question_count, columns, title, exam_code FROM omr_exams WHERE exam_code = $1`, headerResult.ExamCode,
		).Scan(&examID, &questionCount, &columns, &examTitle, &examCode)
	} else {
		lookupErr = fmt.Errorf("exam code on the sheet could not be read clearly")
	}
	if lookupErr != nil || examID == 0 {
		c.JSON(http.StatusUnprocessableEntity, models.ErrorResponse{
			Error: "could not identify which exam this sheet belongs to — retake the photo, or pass exam_id explicitly",
		})
		return
	}

	var unsetAnswers int
	if err := database.DB.QueryRow(ctx,
		`SELECT COUNT(*) FROM omr_questions WHERE omr_exam_id = $1 AND correct_option IS NULL`, examID,
	).Scan(&unsetAnswers); err == nil && unsetAnswers > 0 {
		c.JSON(http.StatusUnprocessableEntity, models.ErrorResponse{Error: "set this token's answer key before evaluating sheets"})
		return
	}

	fullTemplate := omr.BuildTemplate(questionCount, columns)
	result, err := omr.Detect(img, fullTemplate)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, models.ErrorResponse{Error: err.Error()})
		return
	}

	// Match the decoded roll number to a student on this exam's roster.
	var matchedStudentID *int
	var matchedStudentName string
	if !result.RollAmbiguous {
		var sid int
		var name string
		if err := database.DB.QueryRow(ctx,
			`SELECT id, name FROM omr_students WHERE omr_exam_id = $1 AND roll_number = $2`, examID, result.RollNumber,
		).Scan(&sid, &name); err == nil {
			matchedStudentID = &sid
			matchedStudentName = name
		}
	}

	// Score against the answer key.
	qRows, err := database.DB.Query(ctx, `SELECT question_number, correct_option FROM omr_questions WHERE omr_exam_id = $1`, examID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	correctByQ := make(map[int]int)
	for qRows.Next() {
		var qn, opt int
		if err := qRows.Scan(&qn, &opt); err == nil {
			correctByQ[qn] = opt
		}
	}
	qRows.Close()

	outcomes := make([]models.OMRQuestionOutcome, 0, len(result.Questions))
	score := 0
	anyAmbiguous := result.RollAmbiguous || matchedStudentID == nil
	for _, q := range result.Questions {
		correct := correctByQ[q.QuestionNumber]
		isCorrect := !q.Ambiguous && q.SelectedOption == correct
		if isCorrect {
			score++
		}
		if q.Ambiguous {
			anyAmbiguous = true
		}
		outcomes = append(outcomes, models.OMRQuestionOutcome{
			QuestionNumber: q.QuestionNumber,
			SelectedOption: q.SelectedOption,
			CorrectOption:  correct,
			Correct:        isCorrect,
			Ambiguous:      q.Ambiguous,
		})
	}

	status := "scored"
	if anyAmbiguous {
		status = "needs_review"
	}

	rawDetection, _ := json.Marshal(outcomes)
	preview := annotatedPreviewDataURL(omr.Annotate(img, result.Questions, result.BubbleRadiusPx, correctByQ))

	adminIDRaw, _ := c.Get("admin_id")
	adminID, _ := adminIDRaw.(int)
	studentResultID := syncOMRSheetGradebook(ctx, adminID, examID, matchedStudentID, score, questionCount, nil)

	var sheetID int
	var createdAt time.Time
	err = database.DB.QueryRow(ctx,
		`INSERT INTO omr_sheets (omr_exam_id, detected_roll_number, matched_student_id, status, score, total_questions, raw_detection, student_result_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, created_at`,
		examID, result.RollNumber, matchedStudentID, status, score, questionCount, string(rawDetection), studentResultID,
	).Scan(&sheetID, &createdAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to save sheet result"})
		return
	}

	c.JSON(http.StatusCreated, models.OMRSheet{
		ID:                 sheetID,
		OMRExamID:          examID,
		AnnotatedPreview:   preview,
		DetectedRollNumber: result.RollNumber,
		MatchedStudentID:   matchedStudentID,
		MatchedStudentName: matchedStudentName,
		Status:             status,
		Score:              score,
		TotalQuestions:     questionCount,
		DetectedExamCode:   result.ExamCode,
		RollAmbiguous:      result.RollAmbiguous,
		Questions:          outcomes,
		StudentResultID:    studentResultID,
		GradebookSynced:    studentResultID != nil,
		CreatedAt:          createdAt,
	})
}

// AdminListOMRSheets lists scored/flagged sheets for an exam.
func AdminListOMRSheets(c *gin.Context) {
	examID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid exam id"})
		return
	}
	rows, err := database.DB.Query(c.Request.Context(), `
		SELECT sh.id, sh.omr_exam_id, sh.detected_roll_number, sh.matched_student_id,
		       COALESCE(st.name, ''), sh.status, sh.score, sh.total_questions, sh.student_result_id, sh.created_at
		FROM omr_sheets sh
		LEFT JOIN omr_students st ON st.id = sh.matched_student_id
		WHERE sh.omr_exam_id = $1
		ORDER BY sh.created_at DESC`, examID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	sheets := []models.OMRSheet{}
	for rows.Next() {
		var s models.OMRSheet
		if err := rows.Scan(&s.ID, &s.OMRExamID, &s.DetectedRollNumber, &s.MatchedStudentID,
			&s.MatchedStudentName, &s.Status, &s.Score, &s.TotalQuestions, &s.StudentResultID, &s.CreatedAt); err == nil {
			s.GradebookSynced = s.StudentResultID != nil
			sheets = append(sheets, s)
		}
	}
	c.JSON(http.StatusOK, sheets)
}

// AdminGetOMRSheet returns one sheet with its full per-question detail, for
// the review/correction UI.
func AdminGetOMRSheet(c *gin.Context) {
	examID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid exam id"})
		return
	}
	sheetID, err := strconv.Atoi(c.Param("sheetId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid sheet id"})
		return
	}

	var s models.OMRSheet
	var rawDetection string
	err = database.DB.QueryRow(c.Request.Context(), `
		SELECT sh.id, sh.omr_exam_id, sh.detected_roll_number, sh.matched_student_id,
		       COALESCE(st.name, ''), sh.status, sh.score, sh.total_questions, sh.raw_detection, sh.student_result_id, sh.created_at
		FROM omr_sheets sh
		LEFT JOIN omr_students st ON st.id = sh.matched_student_id
		WHERE sh.id = $1 AND sh.omr_exam_id = $2`, sheetID, examID,
	).Scan(&s.ID, &s.OMRExamID, &s.DetectedRollNumber, &s.MatchedStudentID,
		&s.MatchedStudentName, &s.Status, &s.Score, &s.TotalQuestions, &rawDetection, &s.StudentResultID, &s.CreatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "sheet not found"})
		return
	}
	_ = json.Unmarshal([]byte(rawDetection), &s.Questions)
	s.GradebookSynced = s.StudentResultID != nil
	c.JSON(http.StatusOK, s)
}

// AdminDeleteOMRSheet removes a scored sheet's detection record and (if one
// was posted) the student_results row it created, so a duplicate or
// mis-scanned upload doesn't linger in the student's Results.
func AdminDeleteOMRSheet(c *gin.Context) {
	examID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid exam id"})
		return
	}
	sheetID, err := strconv.Atoi(c.Param("sheetId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid sheet id"})
		return
	}

	ctx := c.Request.Context()

	var studentResultID *int
	err = database.DB.QueryRow(ctx,
		`SELECT student_result_id FROM omr_sheets WHERE id = $1 AND omr_exam_id = $2`,
		sheetID, examID,
	).Scan(&studentResultID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "sheet not found"})
		return
	}

	if _, err := database.DB.Exec(ctx, `DELETE FROM omr_sheets WHERE id = $1`, sheetID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to delete sheet"})
		return
	}
	if studentResultID != nil {
		_, _ = database.DB.Exec(ctx, `DELETE FROM student_results WHERE id = $1`, *studentResultID)
	}

	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// AdminUpdateOMRSheet applies manual corrections to a misread sheet —
// per-question selected-option overrides and/or a corrected roster match —
// recomputes the score, and re-syncs the linked student_results row (or
// creates one now, if the correction fixed a previously-unmatched roll).
func AdminUpdateOMRSheet(c *gin.Context) {
	examID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid exam id"})
		return
	}
	sheetID, err := strconv.Atoi(c.Param("sheetId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid sheet id"})
		return
	}
	var req models.UpdateOMRSheetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	ctx := c.Request.Context()

	var rawDetection string
	var matchedStudentID *int
	var totalQuestions int
	var existingResultID *int
	err = database.DB.QueryRow(ctx,
		`SELECT raw_detection, matched_student_id, total_questions, student_result_id FROM omr_sheets WHERE id = $1 AND omr_exam_id = $2`,
		sheetID, examID,
	).Scan(&rawDetection, &matchedStudentID, &totalQuestions, &existingResultID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "sheet not found"})
		return
	}

	var outcomes []models.OMRQuestionOutcome
	if err := json.Unmarshal([]byte(rawDetection), &outcomes); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "could not read stored detection"})
		return
	}

	corrected := make(map[int]int, len(req.Corrections))
	for _, corr := range req.Corrections {
		corrected[corr.QuestionNumber] = corr.CorrectedOption
	}

	score := 0
	stillAmbiguous := false
	for i, o := range outcomes {
		if newOption, ok := corrected[o.QuestionNumber]; ok {
			outcomes[i].SelectedOption = newOption
			outcomes[i].Ambiguous = false
			outcomes[i].Correct = newOption == o.CorrectOption
		}
		if outcomes[i].Correct {
			score++
		}
		if outcomes[i].Ambiguous {
			stillAmbiguous = true
		}
	}

	var matchedStudentName string
	if req.MatchedStudentID != nil {
		matchedStudentID = req.MatchedStudentID
	}
	if matchedStudentID != nil {
		_ = database.DB.QueryRow(ctx, `SELECT name FROM omr_students WHERE id = $1`, *matchedStudentID).Scan(&matchedStudentName)
	} else {
		stillAmbiguous = true
	}

	status := "scored"
	if stillAmbiguous {
		status = "needs_review"
	}

	adminIDRaw, _ := c.Get("admin_id")
	adminID, _ := adminIDRaw.(int)
	studentResultID := syncOMRSheetGradebook(ctx, adminID, examID, matchedStudentID, score, totalQuestions, existingResultID)

	newRaw, _ := json.Marshal(outcomes)
	if _, err := database.DB.Exec(ctx,
		`UPDATE omr_sheets SET matched_student_id = $1, status = $2, score = $3, raw_detection = $4, student_result_id = $5 WHERE id = $6`,
		matchedStudentID, status, score, string(newRaw), studentResultID, sheetID,
	); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to save corrections"})
		return
	}

	var detectedRoll string
	var createdAt time.Time
	_ = database.DB.QueryRow(ctx, `SELECT detected_roll_number, created_at FROM omr_sheets WHERE id = $1`, sheetID).
		Scan(&detectedRoll, &createdAt)

	c.JSON(http.StatusOK, models.OMRSheet{
		ID:                 sheetID,
		OMRExamID:          examID,
		DetectedRollNumber: detectedRoll,
		MatchedStudentID:   matchedStudentID,
		MatchedStudentName: matchedStudentName,
		Status:             status,
		Score:              score,
		TotalQuestions:     totalQuestions,
		Questions:          outcomes,
		StudentResultID:    studentResultID,
		GradebookSynced:    studentResultID != nil,
		CreatedAt:          createdAt,
	})
}
