package omr

import (
	"bufio"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
)

const (
	// darkThreshold: grayscale value (0-255) below which a pixel counts as
	// "ink" when searching for fiducial markers or sampling bubble fill.
	darkThreshold = 110

	// markerSearchFraction: fraction of the image's width/height searched
	// in each corner quadrant for a fiducial marker. The sheet should fill
	// most of the frame, so a generous quadrant is enough without picking
	// up unrelated dark content from elsewhere on the page.
	markerSearchFraction = 0.35

	// A question is "confidently marked" when its darkest bubble's fill
	// ratio clears this absolute floor AND beats the next-darkest bubble by
	// at least fillMargin. Anything short of that is flagged, not guessed.
	fillFloor  = 0.35
	fillMargin = 0.15
)

// BubbleResult is the decoded state of one option/digit bubble.
type BubbleResult struct {
	Value     int // Option (1-4) or digit-value (0-9), matching the source bubble
	FillRatio float64
}

// QuestionResult is the decoded outcome for one question.
type QuestionResult struct {
	QuestionNumber int
	SelectedOption int // 0 = none confidently detected
	Ambiguous      bool
	Options        []BubbleResult
}

// DigitResult is the decoded outcome for one digit column (roll number or exam code).
type DigitResult struct {
	Digit         int
	SelectedValue int // -1 = none confidently detected
	Ambiguous     bool
}

// DetectionResult is the full decoded sheet.
type DetectionResult struct {
	RollNumber        string
	RollAmbiguous     bool
	ExamCode          string
	ExamCodeAmbiguous bool
	Questions         []QuestionResult
}

// DecodeImage reads a JPEG or PNG from r.
func DecodeImage(r io.Reader) (image.Image, error) {
	img, _, err := image.Decode(bufio.NewReader(r))
	if err != nil {
		return nil, fmt.Errorf("omr: unable to decode image: %w", err)
	}
	return img, nil
}

// Detect locates the template's fiducial markers in img, builds the
// homography from template mm-space to photo pixel-space, and decodes every
// roll/exam-code digit and question bubble.
func Detect(img image.Image, t Template) (DetectionResult, error) {
	gray := toGray(img)

	markerPx, err := findMarkers(gray, t)
	if err != nil {
		return DetectionResult{}, err
	}

	h, err := ComputeHomography(t.Markers, markerPx)
	if err != nil {
		return DetectionResult{}, err
	}

	// Bubble radius in pixels: derive from the marker spacing so it scales
	// with the photo's resolution/crop instead of a fixed pixel count.
	scale := pixelsPerMM(t, markerPx)
	bubbleRadiusPx := (BubbleDiameterMM / 2) * scale
	if bubbleRadiusPx < 2 {
		bubbleRadiusPx = 2
	}

	result := DetectionResult{}

	rollDigits := decodeDigitGrid(gray, h, t.RollBubbles, rollDigitCount, bubbleRadiusPx)
	result.RollNumber, result.RollAmbiguous = digitsToString(rollDigits)

	examDigits := decodeDigitGrid(gray, h, t.ExamCodeBubbles, ExamCodeDigitCount, bubbleRadiusPx)
	result.ExamCode, result.ExamCodeAmbiguous = digitsToString(examDigits)

	result.Questions = decodeQuestions(gray, h, t.QuestionBubbles, bubbleRadiusPx)

	return result, nil
}

// toGray converts any image.Image to an *image.Gray for fast, uniform pixel access.
func toGray(img image.Image) *image.Gray {
	if g, ok := img.(*image.Gray); ok {
		return g
	}
	b := img.Bounds()
	gray := image.NewGray(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			gray.Set(x, y, img.At(x, y))
		}
	}
	return gray
}

// findMarkers searches each of the 4 corner quadrants of the photo for the
// centroid of the darkest pixel cluster, returning pixel coordinates in the
// same [TL,TR,BL,BR] order as t.Markers.
func findMarkers(gray *image.Gray, t Template) ([4]Point, error) {
	b := gray.Bounds()
	w, h := b.Dx(), b.Dy()
	qw := int(float64(w) * markerSearchFraction)
	qh := int(float64(h) * markerSearchFraction)

	quadrants := [4]image.Rectangle{
		image.Rect(b.Min.X, b.Min.Y, b.Min.X+qw, b.Min.Y+qh), // top-left
		image.Rect(b.Max.X-qw, b.Min.Y, b.Max.X, b.Min.Y+qh), // top-right
		image.Rect(b.Min.X, b.Max.Y-qh, b.Min.X+qw, b.Max.Y), // bottom-left
		image.Rect(b.Max.X-qw, b.Max.Y-qh, b.Max.X, b.Max.Y), // bottom-right
	}

	var out [4]Point
	for i, q := range quadrants {
		cx, cy, count, err := largestDarkBlobCentroid(gray, q)
		if err != nil || count < 20 {
			return out, fmt.Errorf("omr: could not find a corner marker (%d/4 found) — retake the photo with all 4 corners visible and well lit", i)
		}
		out[i] = Point{XMM: cx, YMM: cy}
	}
	return out, nil
}

// largestDarkBlobCentroid finds the largest 4-connected component of
// pixels darker than darkThreshold within rect and returns its centroid
// (in pixel coordinates) and pixel count. The fiducial marker is a large
// solid square, so it dominates any nearby dark content (bubble fills,
// stray marks) as long as those are picked by size rather than lumped
// together with a whole-region centroid.
func largestDarkBlobCentroid(gray *image.Gray, rect image.Rectangle) (float64, float64, int, error) {
	w, h := rect.Dx(), rect.Dy()
	if w <= 0 || h <= 0 {
		return 0, 0, 0, fmt.Errorf("empty search region")
	}
	visited := make([]bool, w*h)
	idx := func(x, y int) int { return (y-rect.Min.Y)*w + (x - rect.Min.X) }

	bestCount := 0
	var bestSumX, bestSumY float64

	type pt struct{ x, y int }
	queue := make([]pt, 0, 256)

	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if visited[idx(x, y)] || gray.GrayAt(x, y).Y >= darkThreshold {
				continue
			}
			// BFS this component.
			queue = queue[:0]
			queue = append(queue, pt{x, y})
			visited[idx(x, y)] = true
			count := 0
			var sumX, sumY float64
			for len(queue) > 0 {
				p := queue[len(queue)-1]
				queue = queue[:len(queue)-1]
				count++
				sumX += float64(p.x)
				sumY += float64(p.y)
				neighbors := [4]pt{{p.x - 1, p.y}, {p.x + 1, p.y}, {p.x, p.y - 1}, {p.x, p.y + 1}}
				for _, n := range neighbors {
					if n.x < rect.Min.X || n.x >= rect.Max.X || n.y < rect.Min.Y || n.y >= rect.Max.Y {
						continue
					}
					if visited[idx(n.x, n.y)] {
						continue
					}
					if gray.GrayAt(n.x, n.y).Y >= darkThreshold {
						continue
					}
					visited[idx(n.x, n.y)] = true
					queue = append(queue, n)
				}
			}
			if count > bestCount {
				bestCount = count
				bestSumX, bestSumY = sumX, sumY
			}
		}
	}
	if bestCount == 0 {
		return 0, 0, 0, fmt.Errorf("no dark pixels found")
	}
	return bestSumX / float64(bestCount), bestSumY / float64(bestCount), bestCount, nil
}

// pixelsPerMM estimates the photo's scale from the average of the 4
// detected marker-to-marker distances vs. their known mm distances.
func pixelsPerMM(t Template, markerPx [4]Point) float64 {
	mmDist := distance(t.Markers[0], t.Markers[1]) // TL-TR
	pxDist := distance(markerPx[0], markerPx[1])
	if mmDist == 0 {
		return 1
	}
	return pxDist / mmDist
}

func distance(a, b Point) float64 {
	dx := a.XMM - b.XMM
	dy := a.YMM - b.YMM
	return sqrt(dx*dx + dy*dy)
}

func sqrt(v float64) float64 {
	if v <= 0 {
		return 0
	}
	// Newton's method — avoids importing math for a single call site.
	x := v
	for i := 0; i < 20; i++ {
		x = 0.5 * (x + v/x)
	}
	return x
}

// fillRatioAt samples a bubbleRadiusPx-radius square around (px,py) in gray
// and returns the fraction of pixels darker than darkThreshold.
func fillRatioAt(gray *image.Gray, px, py, bubbleRadiusPx float64) float64 {
	b := gray.Bounds()
	x0 := int(px - bubbleRadiusPx)
	x1 := int(px + bubbleRadiusPx)
	y0 := int(py - bubbleRadiusPx)
	y1 := int(py + bubbleRadiusPx)
	if x0 < b.Min.X {
		x0 = b.Min.X
	}
	if y0 < b.Min.Y {
		y0 = b.Min.Y
	}
	if x1 > b.Max.X {
		x1 = b.Max.X
	}
	if y1 > b.Max.Y {
		y1 = b.Max.Y
	}
	if x1 <= x0 || y1 <= y0 {
		return 0
	}
	var dark, total int
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			total++
			if gray.GrayAt(x, y).Y < darkThreshold {
				dark++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(dark) / float64(total)
}

// decodeDigitGrid decodes each digit column of a roll-number/exam-code grid.
func decodeDigitGrid(gray *image.Gray, h Homography, bubbles []DigitBubble, digitCount int, bubbleRadiusPx float64) []DigitResult {
	byDigit := make([][]DigitBubble, digitCount)
	for _, bub := range bubbles {
		if bub.Digit >= 0 && bub.Digit < digitCount {
			byDigit[bub.Digit] = append(byDigit[bub.Digit], bub)
		}
	}

	results := make([]DigitResult, digitCount)
	for d := 0; d < digitCount; d++ {
		best, second := -1, -1
		bestFill, secondFill := 0.0, 0.0
		for _, bub := range byDigit[d] {
			px, py := h.Apply(bub.Center)
			fill := fillRatioAt(gray, px, py, bubbleRadiusPx)
			if fill > bestFill {
				second, secondFill = best, bestFill
				best, bestFill = bub.Value, fill
			} else if fill > secondFill {
				second, secondFill = bub.Value, fill
			}
		}
		_ = second
		ambiguous := best == -1 || bestFill < fillFloor || (bestFill-secondFill) < fillMargin
		results[d] = DigitResult{Digit: d, SelectedValue: best, Ambiguous: ambiguous}
	}
	return results
}

func digitsToString(digits []DigitResult) (string, bool) {
	out := make([]byte, len(digits))
	ambiguous := false
	for i, d := range digits {
		if d.Ambiguous || d.SelectedValue < 0 {
			out[i] = '?'
			ambiguous = true
			continue
		}
		out[i] = byte('0' + d.SelectedValue)
	}
	return string(out), ambiguous
}

// decodeQuestions decodes every question's 4 option bubbles.
func decodeQuestions(gray *image.Gray, h Homography, bubbles []QuestionBubble, bubbleRadiusPx float64) []QuestionResult {
	byQuestion := make(map[int][]QuestionBubble)
	var order []int
	for _, bub := range bubbles {
		if _, seen := byQuestion[bub.QuestionNumber]; !seen {
			order = append(order, bub.QuestionNumber)
		}
		byQuestion[bub.QuestionNumber] = append(byQuestion[bub.QuestionNumber], bub)
	}

	results := make([]QuestionResult, 0, len(order))
	for _, qn := range order {
		opts := byQuestion[qn]
		qr := QuestionResult{QuestionNumber: qn}
		best, second := 0, 0
		bestFill, secondFill := 0.0, 0.0
		for _, bub := range opts {
			px, py := h.Apply(bub.Center)
			fill := fillRatioAt(gray, px, py, bubbleRadiusPx)
			qr.Options = append(qr.Options, BubbleResult{Value: bub.Option, FillRatio: fill})
			if fill > bestFill {
				second, secondFill = best, bestFill
				best, bestFill = bub.Option, fill
			} else if fill > secondFill {
				second, secondFill = bub.Option, fill
			}
		}
		_ = second
		if best == 0 || bestFill < fillFloor || (bestFill-secondFill) < fillMargin {
			qr.Ambiguous = true
		} else {
			qr.SelectedOption = best
		}
		results = append(results, qr)
	}
	return results
}
