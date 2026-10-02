package omr

import (
	"image"
	"image/color"
	"image/draw"
)

var (
	colorCorrect = color.RGBA{34, 197, 94, 255} // emerald-500 — marked and correct, or the right answer when the student missed it
	colorWrong   = color.RGBA{239, 68, 68, 255} // red-500 — marked and incorrect
)

// Annotate draws a colored ring over each question's bubbles onto a copy of
// the scanned sheet: the student's marked bubble in green (correct) or red
// (incorrect), plus a thin green ring around the correct bubble whenever the
// student's mark missed it — so a teacher can see both what was marked and
// what should have been marked without opening the original answer key.
func Annotate(img image.Image, questions []QuestionResult, bubbleRadiusPx float64, correctByQuestion map[int]int) image.Image {
	b := img.Bounds()
	out := image.NewRGBA(b)
	draw.Draw(out, b, img, b.Min, draw.Src)

	radius := bubbleRadiusPx
	if radius < 4 {
		radius = 4
	}

	for _, q := range questions {
		correct := correctByQuestion[q.QuestionNumber]
		for _, opt := range q.Options {
			switch {
			case !q.Ambiguous && q.SelectedOption == opt.Value && opt.Value == correct:
				drawRing(out, opt.PixelX, opt.PixelY, radius, colorCorrect, 3)
			case !q.Ambiguous && q.SelectedOption == opt.Value && opt.Value != correct:
				drawRing(out, opt.PixelX, opt.PixelY, radius, colorWrong, 3)
			case opt.Value == correct:
				drawRing(out, opt.PixelX, opt.PixelY, radius, colorCorrect, 2)
			}
		}
	}

	return out
}

// drawRing draws a circular outline of the given thickness centered at (cx,
// cy) — an outline rather than a fill so the original bubble ink underneath
// stays visible.
func drawRing(img *image.RGBA, cx, cy, radius float64, col color.RGBA, thickness int) {
	outer := radius + float64(thickness)
	inner := radius
	x0 := int(cx - outer - 1)
	x1 := int(cx + outer + 1)
	y0 := int(cy - outer - 1)
	y1 := int(cy + outer + 1)
	b := img.Bounds()
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
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			dx, dy := float64(x)-cx, float64(y)-cy
			d := sqrt(dx*dx + dy*dy)
			if d >= inner && d <= outer {
				img.Set(x, y, col)
			}
		}
	}
}
