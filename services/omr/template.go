// Package omr builds the deterministic OMR sheet layout (in millimeters)
// and decodes photographed sheets against that same layout. The layout is
// computed once here so the printable sheet (rendered in the admin web app
// from the /template endpoint) and the scorer (this package, run against an
// uploaded photo) always agree on where every bubble is.
package omr

import "fmt"

const (
	PageWidthMM  = 210.0 // A4 portrait
	PageHeightMM = 297.0

	frameMarginMM = 12.0
	MarkerSizeMM  = 8.0
	// Horizontal gap between each corner marker's own edge and the content
	// area beside it, and vertical gap above the content's bottom edge from
	// the bottom markers. The top edge deliberately does *not* use this (see
	// contentTop in BuildTemplate) — the warning bar sits level with the top
	// markers instead of stacked below them, reclaiming a full MarkerSizeMM
	// of page height instead of leaving it as a dead band.
	contentPadMM = 4.0

	BubbleDiameterMM = 4.0

	// Header layout, top to bottom: warning bar, title, then one row
	// holding the Roll Number and Subject Code grids (bubbled/decoded) side
	// by side with a Class and Set Code box (rendered by the frontend for
	// visual match to the reference sheet, but not decoded — see
	// ClassBoxWidthMM/SetBoxWidthMM below), then a section title before the
	// question grid starts.
	WarningBarHeightMM = 8.0
	TitleHeightMM      = 14.0
	// Tall enough to hold the label strip, one blank spacer row (matching the
	// reference sheet, which leaves a row of empty cells between the label
	// and the first bubbled row), and all 10 digit-value rows
	// (infoLabelHeightMM + 11*rollValueSpacingMM), so the Roll/Subject Code
	// box frames actually contain their own bubbles.
	InfoRowHeightMM = 66.5
	// Room for the "বহুনির্বাচনি অভিক্ষার উত্তরপত্র" title, sitting above the
	// question grid's own header band (which the frontend carves out of the
	// bottom of this space — see headerTop in OmrSheet.tsx) — tall enough
	// that the title text gets clear padding above and below it instead of
	// touching the signature box above or the question header below.
	SectionTitleHeightMM = 18.0

	// Height of the label strip at the top of each info-row box (Class, Roll
	// Number, Subject Code, Set Code), above the blank spacer row.
	infoLabelHeightMM = 6.0

	// Left-to-right box widths within the info row — exported so the
	// frontend can lay out the (purely cosmetic) Class and Set Code boxes
	// at the exact same X offsets as the Go-computed Roll/Subject Code grids.
	ClassBoxWidthMM = 24.0
	infoBoxGapMM    = 6.0
	SetBoxWidthMM   = 18.5

	// Subject code: a short bubbled digit code (pre-filled identically on
	// every copy of a given exam's sheet) that lets Evaluate OMR identify
	// which exam a photo belongs to without the admin picking one first.
	ExamCodeDigitCount     = 3
	examCodeDigitSpacingMM = 6.5

	rollDigitCount     = 6 // supports roll numbers up to 6 digits
	rollDigitSpacingMM = 8.0
	rollValueSpacingMM = 5.5 // vertical spacing between 0-9 bubbles in a digit column

	// Rows are spaced to use up the page: a full 100-question/4-column sheet
	// ends just above the bottom markers rather than leaving a dead band,
	// which also gives the scorer a taller row to sample per question.
	questionRowHeightMM = 6.0
	columnGapMM         = 10.0
	// The question-number label and the 4 option bubbles sit in 5 equal-width
	// cells that scale to fill whatever width a column actually has (so
	// 40/60-question sheets with fewer, wider columns don't leave a dead
	// strip of blank space to the right — the reference sheet's tight
	// 4-column packing was only "tight" because it had 4 columns; fewer
	// columns means more width per column, and the bubbles should spread to
	// use it). minOptionSpacingMM floors the cell width so it stays legible
	// (and clears BubbleDiameterMM) if a column ever gets narrow.
	minOptionSpacingMM = 5.0

	// MinColumns/MaxColumns bound the admin-facing column picker.
	MinColumns   = 1
	MaxColumns   = 4
	MinQuestions = 10
	MaxQuestions = 100
)

// Point is a coordinate in millimeters relative to the top-left of the page.
type Point struct {
	XMM float64 `json:"x_mm"`
	YMM float64 `json:"y_mm"`
}

// QuestionBubble is one option bubble (Option 1=A .. 4=D) for one question.
type QuestionBubble struct {
	QuestionNumber int   `json:"question_number"`
	Option         int   `json:"option"`
	Center         Point `json:"center"`
}

// DigitBubble is one digit-value bubble in a digit grid (roll number or
// subject code). Digit is the column (0 = leftmost/most-significant), Value
// is 0-9.
type DigitBubble struct {
	Digit  int   `json:"digit"`
	Value  int   `json:"value"`
	Center Point `json:"center"`
}

// Template is the full deterministic layout for a given question count and
// column count.
type Template struct {
	PageWidthMM      float64 `json:"page_width_mm"`
	PageHeightMM     float64 `json:"page_height_mm"`
	MarkerSizeMM     float64 `json:"marker_size_mm"`
	BubbleDiameterMM float64 `json:"bubble_diameter_mm"`
	Columns          int     `json:"columns"`
	// Markers are marker centers, in a fixed order: top-left, top-right, bottom-left, bottom-right.
	Markers            [4]Point         `json:"markers"`
	RollDigitCount     int              `json:"roll_digit_count"`
	RollBubbles        []DigitBubble    `json:"roll_bubbles"`
	ExamCodeDigitCount int              `json:"exam_code_digit_count"`
	ExamCodeBubbles    []DigitBubble    `json:"exam_code_bubbles"`
	QuestionBubbles    []QuestionBubble `json:"question_bubbles"`
	// ContentLeftMM/ContentTopMM/InfoRowTopMM are exposed so the frontend
	// can position the cosmetic Class/Set Code boxes and header text at the
	// same coordinates this layout uses, without duplicating the margin math.
	ContentLeftMM        float64 `json:"content_left_mm"`
	ContentRightMM       float64 `json:"content_right_mm"`
	ContentTopMM         float64 `json:"content_top_mm"`
	InfoRowTopMM         float64 `json:"info_row_top_mm"`
	QuestionsTopMM       float64 `json:"questions_top_mm"`
	WarningBarHeightMM   float64 `json:"warning_bar_height_mm"`
	TitleHeightMM        float64 `json:"title_height_mm"`
	InfoRowHeightMM      float64 `json:"info_row_height_mm"`
	SectionTitleHeightMM float64 `json:"section_title_height_mm"`
	// Left edges + widths for the 4 side-by-side info-row boxes (Class,
	// Roll, Subject Code, Set), so the frontend can position the cosmetic
	// Class/Set boxes flush against the Go-computed Roll/Subject Code grids
	// without duplicating any spacing math.
	ClassBoxLeftMM        float64 `json:"class_box_left_mm"`
	ClassBoxWidthMM       float64 `json:"class_box_width_mm"`
	RollBoxLeftMM         float64 `json:"roll_box_left_mm"`
	RollBoxWidthMM        float64 `json:"roll_box_width_mm"`
	SubjectCodeBoxLeftMM  float64 `json:"subject_code_box_left_mm"`
	SubjectCodeBoxWidthMM float64 `json:"subject_code_box_width_mm"`
	SetBoxLeftMM          float64 `json:"set_box_left_mm"`
	SetBoxWidthMM         float64 `json:"set_box_width_mm"`
}

// SuggestColumns picks a sensible default column count for a question count,
// matching the same thresholds IsScannable uses to judge feasibility.
func SuggestColumns(questionCount int) int {
	switch {
	case questionCount > 75:
		return 4
	case questionCount > 50:
		return 3
	case questionCount > 25:
		return 2
	default:
		return 1
	}
}

// questionLabelAndOptionSpacing scales the question-number label cell and
// the 4 option cells to 5 *equal-width* cells filling a column's content
// width (columnGapMM already excluded by the caller), so fewer/wider
// columns (lower question counts) don't leave blank space instead of
// spreading the bubbles out, and the question number and every option
// bubble line up on the same cell width/center.
func questionLabelAndOptionSpacing(columnContentWidth float64) (labelWidth, optionSpacing float64) {
	cellWidth := columnContentWidth / 5
	if cellWidth < minOptionSpacingMM {
		cellWidth = minOptionSpacingMM
	}
	return cellWidth, cellWidth
}

// columnContentWidth splits availableWidth into `columns` equal content
// widths with columnGapMM of gap *between* them only (columns-1 gaps, not
// one per column) — a trailing gap after the last column would leave it
// short of contentRight, misaligning its right edge against the other
// full-width elements on the sheet (info row, signature box, etc).
func columnContentWidth(availableWidth float64, columns int) float64 {
	return (availableWidth - columnGapMM*float64(columns-1)) / float64(columns)
}

// IsScannable reports whether questionCount questions laid out in columns
// columns will fit on one A4 page with rows tall enough to reliably detect
// bubble fills. Mirrors the reference tool's live "this OMR is scannable"
// check so the admin UI can validate before printing.
func IsScannable(questionCount, columns int) (bool, string) {
	if questionCount < MinQuestions || questionCount > MaxQuestions {
		return false, fmt.Sprintf("question count must be between %d and %d", MinQuestions, MaxQuestions)
	}
	if columns < MinColumns || columns > MaxColumns {
		return false, fmt.Sprintf("columns must be between %d and %d", MinColumns, MaxColumns)
	}
	// Level with the bottom markers' own bottom edge, mirroring contentTop —
	// the last question row can run down beside the bottom markers (clear of
	// them horizontally, same as the top) instead of stopping a full
	// MarkerSizeMM+contentPadMM above them and leaving that as a dead band.
	contentBottom := PageHeightMM - frameMarginMM
	questionsTop := frameMarginMM + WarningBarHeightMM + TitleHeightMM + InfoRowHeightMM + SectionTitleHeightMM
	availableHeight := contentBottom - questionsTop
	rowsPerColumn := (questionCount + columns - 1) / columns
	if float64(rowsPerColumn)*questionRowHeightMM > availableHeight {
		return false, "too many questions for this many columns to fit on one page — add a column or reduce questions"
	}
	availableWidth := PageWidthMM - 2*(frameMarginMM+MarkerSizeMM+contentPadMM)
	_, optionSpacing := questionLabelAndOptionSpacing(columnContentWidth(availableWidth, columns))
	if optionSpacing < minOptionSpacingMM {
		return false, "too many columns for the page width — remove a column"
	}
	return true, ""
}

// BuildTemplate computes the deterministic sheet layout for questionCount
// questions with 4 options each (A-D), arranged in the given number of
// columns. It's a pure function so the same layout can be recomputed
// independently by the print page and by the scorer. Pass columns <= 0 to
// use SuggestColumns' default.
func BuildTemplate(questionCount, columns int) Template {
	if questionCount < 1 {
		questionCount = 1
	}
	if columns <= 0 {
		columns = SuggestColumns(questionCount)
	}
	if columns < MinColumns {
		columns = MinColumns
	}
	if columns > MaxColumns {
		columns = MaxColumns
	}

	markerHalf := MarkerSizeMM / 2
	contentLeft := frameMarginMM + MarkerSizeMM + contentPadMM
	contentRight := PageWidthMM - frameMarginMM - MarkerSizeMM - contentPadMM
	// Level with the top markers' own top edge (frameMarginMM), not stacked
	// below them — the warning bar sits in the same row as the markers,
	// clear of them horizontally (contentLeft already skips past their
	// width), so no MarkerSizeMM of otherwise-dead vertical space is spent
	// pushing content below the markers before it even starts.
	contentTop := frameMarginMM
	infoRowTop := contentTop + WarningBarHeightMM + TitleHeightMM
	questionsTop := infoRowTop + InfoRowHeightMM + SectionTitleHeightMM

	t := Template{
		PageWidthMM:        PageWidthMM,
		PageHeightMM:       PageHeightMM,
		MarkerSizeMM:       MarkerSizeMM,
		BubbleDiameterMM:   BubbleDiameterMM,
		Columns:            columns,
		RollDigitCount:     rollDigitCount,
		ExamCodeDigitCount: ExamCodeDigitCount,
		Markers: [4]Point{
			{XMM: frameMarginMM + markerHalf, YMM: frameMarginMM + markerHalf},                              // top-left
			{XMM: PageWidthMM - frameMarginMM - markerHalf, YMM: frameMarginMM + markerHalf},                // top-right
			{XMM: frameMarginMM + markerHalf, YMM: PageHeightMM - frameMarginMM - markerHalf},               // bottom-left
			{XMM: PageWidthMM - frameMarginMM - markerHalf, YMM: PageHeightMM - frameMarginMM - markerHalf}, // bottom-right
		},
		ContentLeftMM:        contentLeft,
		ContentRightMM:       contentRight,
		ContentTopMM:         contentTop,
		InfoRowTopMM:         infoRowTop,
		QuestionsTopMM:       questionsTop,
		WarningBarHeightMM:   WarningBarHeightMM,
		TitleHeightMM:        TitleHeightMM,
		InfoRowHeightMM:      InfoRowHeightMM,
		SectionTitleHeightMM: SectionTitleHeightMM,
	}

	// Info row, left to right: Class box (cosmetic, frontend-only) — Roll
	// Number grid — Subject Code grid — Set Code box (cosmetic).
	//
	// Bubbles are centered inside their grid cell, both horizontally (half a
	// digit-column spacing in from the box's left edge) and vertically (half
	// a value-row spacing below the label strip) — so the box frame and the
	// cell divider lines drawn from these coordinates land *between* bubbles
	// instead of slicing through the first row/column of them.
	bubblesTop := infoRowTop + infoLabelHeightMM + rollValueSpacingMM + rollValueSpacingMM/2

	rollBoxLeft := contentLeft + ClassBoxWidthMM + infoBoxGapMM
	rollLeft := rollBoxLeft + rollDigitSpacingMM/2
	for d := 0; d < rollDigitCount; d++ {
		colX := rollLeft + float64(d)*rollDigitSpacingMM
		for v := 0; v < 10; v++ {
			t.RollBubbles = append(t.RollBubbles, DigitBubble{
				Digit: d,
				Value: v,
				Center: Point{
					XMM: colX,
					YMM: bubblesTop + float64(v)*rollValueSpacingMM,
				},
			})
		}
	}
	rollWidth := float64(rollDigitCount) * rollDigitSpacingMM

	examCodeBoxLeft := rollBoxLeft + rollWidth + infoBoxGapMM
	examCodeLeft := examCodeBoxLeft + examCodeDigitSpacingMM/2
	for d := 0; d < ExamCodeDigitCount; d++ {
		colX := examCodeLeft + float64(d)*examCodeDigitSpacingMM
		for v := 0; v < 10; v++ {
			t.ExamCodeBubbles = append(t.ExamCodeBubbles, DigitBubble{
				Digit: d,
				Value: v,
				Center: Point{
					XMM: colX,
					YMM: bubblesTop + float64(v)*rollValueSpacingMM,
				},
			})
		}
	}
	examCodeWidth := float64(ExamCodeDigitCount) * examCodeDigitSpacingMM

	t.ClassBoxLeftMM = contentLeft
	t.ClassBoxWidthMM = ClassBoxWidthMM
	t.RollBoxLeftMM = rollBoxLeft
	t.RollBoxWidthMM = rollWidth
	t.SubjectCodeBoxLeftMM = examCodeBoxLeft
	t.SubjectCodeBoxWidthMM = examCodeWidth
	t.SetBoxLeftMM = examCodeBoxLeft + examCodeWidth + infoBoxGapMM
	t.SetBoxWidthMM = SetBoxWidthMM

	// Question grid: split into the requested number of columns so larger
	// question counts still fit one A4 page. Each column gets an equal share
	// of the width with columnGapMM between columns (not after the last one),
	// so the last column's content reaches all the way to contentRight —
	// matching the other full-width elements on the sheet (info row,
	// signature box, etc) instead of stopping a gap's-width short of it.
	availableWidth := contentRight - contentLeft
	colContentWidth := columnContentWidth(availableWidth, columns)
	colStride := colContentWidth + columnGapMM
	rowsPerColumn := (questionCount + columns - 1) / columns
	labelWidth, optionSpacing := questionLabelAndOptionSpacing(colContentWidth)

	q := 1
	for col := 0; col < columns && q <= questionCount; col++ {
		colX := contentLeft + float64(col)*colStride
		for row := 0; row < rowsPerColumn && q <= questionCount; row++ {
			rowY := questionsTop + float64(row)*questionRowHeightMM
			for opt := 1; opt <= 4; opt++ {
				t.QuestionBubbles = append(t.QuestionBubbles, QuestionBubble{
					QuestionNumber: q,
					Option:         opt,
					Center: Point{
						// Centered within its own equal-width cell, not pinned
						// to the cell's left edge — matches the label cell,
						// whose text the frontend already centers.
						XMM: colX + labelWidth + (float64(opt-1)+0.5)*optionSpacing,
						YMM: rowY,
					},
				})
			}
			q++
		}
	}
	return t
}
