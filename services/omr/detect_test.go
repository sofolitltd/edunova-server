package omr

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

// renderSyntheticSheet draws a "photograph" of a filled-in sheet at
// scalePxPerMM pixels-per-mm, so the detection pipeline can be tested
// without a real printer/camera: markers, the exam code bubbles (filled per
// examCode), one filled bubble per roll digit (rollNumber), and one filled
// bubble per question (answers, 1-based option index, 0 = leave blank).
func renderSyntheticSheet(test *testing.T, tpl Template, examCode, rollNumber string, answers map[int]int, scale float64) image.Image {
	w := int(tpl.PageWidthMM * scale)
	h := int(tpl.PageHeightMM * scale)
	img := image.NewGray(image.Rect(0, 0, w, h))
	// White background.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetGray(x, y, color.Gray{Y: 255})
		}
	}

	fillSquare := func(cx, cy, sizeMM float64) {
		half := sizeMM * scale / 2
		px, py := cx*scale, cy*scale
		for y := int(py - half); y < int(py+half); y++ {
			for x := int(px - half); x < int(px+half); x++ {
				if x >= 0 && x < w && y >= 0 && y < h {
					img.SetGray(x, y, color.Gray{Y: 0})
				}
			}
		}
	}
	fillCircle := func(cx, cy, diameterMM float64) {
		r := diameterMM * scale / 2
		px, py := cx*scale, cy*scale
		for y := int(py - r); y < int(py+r); y++ {
			for x := int(px - r); x < int(px+r); x++ {
				dx, dy := float64(x)-px, float64(y)-py
				if dx*dx+dy*dy <= r*r && x >= 0 && x < w && y >= 0 && y < h {
					img.SetGray(x, y, color.Gray{Y: 0})
				}
			}
		}
	}

	for _, m := range tpl.Markers {
		fillSquare(m.XMM, m.YMM, tpl.MarkerSizeMM)
	}
	for _, b := range tpl.ExamCodeBubbles {
		if b.Digit < len(examCode) && examCode[b.Digit]-'0' == byte(b.Value) {
			fillCircle(b.Center.XMM, b.Center.YMM, tpl.BubbleDiameterMM)
		}
	}
	for _, b := range tpl.RollBubbles {
		if b.Digit < len(rollNumber) && rollNumber[b.Digit]-'0' == byte(b.Value) {
			fillCircle(b.Center.XMM, b.Center.YMM, tpl.BubbleDiameterMM)
		}
	}
	for _, b := range tpl.QuestionBubbles {
		if answers[b.QuestionNumber] == b.Option {
			fillCircle(b.Center.XMM, b.Center.YMM, tpl.BubbleDiameterMM)
		}
	}

	// Round-trip through JPEG to mimic real photo compression artifacts.
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		test.Fatalf("failed to encode synthetic sheet: %v", err)
	}
	decoded, err := jpeg.Decode(&buf)
	if err != nil {
		test.Fatalf("failed to decode synthetic sheet: %v", err)
	}
	return decoded
}

func TestDetect_DecodesRollExamCodeAndAnswers(t *testing.T) {
	tpl := BuildTemplate(20, 2)
	examCode := "141"
	rollNumber := "000123"
	answers := map[int]int{}
	for q := 1; q <= 20; q++ {
		answers[q] = (q % 4) + 1 // deterministic varied pattern across A-D
	}

	img := renderSyntheticSheet(t, tpl, examCode, rollNumber, answers, 8)

	result, err := Detect(img, tpl)
	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}

	if result.ExamCode != examCode {
		t.Errorf("exam code = %q, want %q (ambiguous=%v)", result.ExamCode, examCode, result.ExamCodeAmbiguous)
	}
	if result.ExamCodeAmbiguous {
		t.Errorf("exam code unexpectedly ambiguous")
	}
	if result.RollNumber != rollNumber {
		t.Errorf("roll number = %q, want %q (ambiguous=%v)", result.RollNumber, rollNumber, result.RollAmbiguous)
	}
	if result.RollAmbiguous {
		t.Errorf("roll number unexpectedly ambiguous")
	}
	if len(result.Questions) != 20 {
		t.Fatalf("got %d questions, want 20", len(result.Questions))
	}
	for _, q := range result.Questions {
		want := answers[q.QuestionNumber]
		if q.Ambiguous {
			t.Errorf("question %d unexpectedly ambiguous", q.QuestionNumber)
			continue
		}
		if q.SelectedOption != want {
			t.Errorf("question %d = option %d, want %d", q.QuestionNumber, q.SelectedOption, want)
		}
	}
}

func TestDetect_FlagsBlankQuestionAsAmbiguous(t *testing.T) {
	tpl := BuildTemplate(10, 1)
	answers := map[int]int{1: 1, 2: 2, 3: 0 /* left blank */}
	img := renderSyntheticSheet(t, tpl, "001", "000001", answers, 8)

	result, err := Detect(img, tpl)
	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}
	for _, q := range result.Questions {
		if q.QuestionNumber == 3 && !q.Ambiguous {
			t.Errorf("question 3 was left blank but decoded as option %d instead of ambiguous", q.SelectedOption)
		}
	}
}

func TestDetect_WorksAtLowerPhotoResolution(t *testing.T) {
	tpl := BuildTemplate(50, 3)
	examCode := "999"
	rollNumber := "054321"
	answers := map[int]int{}
	for q := 1; q <= 50; q++ {
		answers[q] = ((q * 3) % 4) + 1
	}
	// ~3.5 px/mm is roughly what an A4 page photographed at ~2000px wide yields.
	img := renderSyntheticSheet(t, tpl, examCode, rollNumber, answers, 3.5)

	result, err := Detect(img, tpl)
	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}
	if result.ExamCode != examCode || result.ExamCodeAmbiguous {
		t.Errorf("exam code = %q (ambiguous=%v), want %q", result.ExamCode, result.ExamCodeAmbiguous, examCode)
	}
	if result.RollNumber != rollNumber || result.RollAmbiguous {
		t.Errorf("roll number = %q (ambiguous=%v), want %q", result.RollNumber, result.RollAmbiguous, rollNumber)
	}
	wrong := 0
	for _, q := range result.Questions {
		if q.Ambiguous || q.SelectedOption != answers[q.QuestionNumber] {
			wrong++
		}
	}
	if wrong > 0 {
		t.Errorf("%d/%d questions decoded incorrectly at low resolution", wrong, len(result.Questions))
	}
}

func TestBuildTemplate_QuestionColumnsFillAvailableWidth(t *testing.T) {
	for _, tc := range []struct {
		questionCount, columns int
	}{
		{40, 2},
		{60, 3},
		{100, 4},
	} {
		tpl := BuildTemplate(tc.questionCount, tc.columns)
		var maxX float64
		for _, b := range tpl.QuestionBubbles {
			if b.Center.XMM > maxX {
				maxX = b.Center.XMM
			}
		}
		// The rightmost bubble's center should land in the final column,
		// close to the page's content edge — not stranded a whole column's
		// width short of it (the bug: fixed label/option spacing regardless
		// of how wide each column actually is).
		slack := tpl.ContentRightMM - maxX
		columnWidth := (tpl.ContentRightMM - tpl.ContentLeftMM) / float64(tc.columns)
		if slack > columnWidth*0.5 {
			t.Errorf("q=%d cols=%d: rightmost question bubble is %.1fmm short of content edge (column width %.1fmm) — question grid isn't filling the page", tc.questionCount, tc.columns, slack, columnWidth)
		}
	}
}
