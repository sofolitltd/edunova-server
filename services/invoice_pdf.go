package services

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"

	"edunova-server/models"
)

// The PDF is drawn directly from the invoice snapshot. fpdf does no complex
// text shaping, so Bangla letters render but conjuncts and vowel signs are not
// reordered; invoice labels and amounts are Latin and unaffected.

//go:embed fonts/NotoSans-Regular.ttf
var fontLatin []byte

//go:embed fonts/NotoSans-Bold.ttf
var fontLatinBold []byte

//go:embed fonts/NotoSansBengali-Regular.ttf
var fontBangla []byte

//go:embed fonts/NotoSansBengali-Bold.ttf
var fontBanglaBold []byte

const (
	familyLatin  = "latin"
	familyBangla = "bangla"

	pageMargin = 12.0
	copyWidth  = 186.0
	copyPad    = 6.0
	innerWidth = copyWidth - 2*copyPad
)

// Bangladesh has no DST, so a fixed offset avoids needing tzdata in the image.
var dhaka = time.FixedZone("BST", 6*3600)

type invoiceDoc struct{ *fpdf.Fpdf }

type textRun struct{ family, text string }

func isBangla(r rune) bool { return (r >= 0x0980 && r <= 0x09FF) || r == 0x200C || r == 0x200D }

// ContainsBangla reports whether s has any Bangla-script character.
func ContainsBangla(s string) bool {
	for _, r := range s {
		if isBangla(r) {
			return true
		}
	}
	return false
}

// splitRuns separates Bangla text from everything else so each run can use
// the font that has its glyphs.
func splitRuns(s string) []textRun {
	var runs []textRun
	var cur strings.Builder
	family := familyLatin
	flush := func() {
		if cur.Len() > 0 {
			runs = append(runs, textRun{family, cur.String()})
			cur.Reset()
		}
	}
	for _, r := range s {
		f := familyLatin
		if isBangla(r) {
			f = familyBangla
		}
		if f != family {
			flush()
			family = f
		}
		cur.WriteRune(r)
	}
	flush()
	return runs
}

func (d *invoiceDoc) font(family string, bold bool, size float64) {
	style := ""
	if bold {
		style = "B"
	}
	d.SetFont(family, style, size)
}

func (d *invoiceDoc) width(s string, bold bool, size float64) float64 {
	w := 0.0
	for _, r := range splitRuns(s) {
		d.font(r.family, bold, size)
		w += d.GetStringWidth(r.text)
	}
	return w
}

// text draws s with its baseline at y.
func (d *invoiceDoc) text(x, y float64, s string, bold bool, size float64) {
	for _, r := range splitRuns(s) {
		d.font(r.family, bold, size)
		d.Text(x, y, r.text)
		x += d.GetStringWidth(r.text)
	}
}

func (d *invoiceDoc) textRight(xRight, y float64, s string, bold bool, size float64) {
	d.text(xRight-d.width(s, bold, size), y, s, bold, size)
}

func (d *invoiceDoc) textCenter(xCenter, y float64, s string, bold bool, size float64) {
	d.text(xCenter-d.width(s, bold, size)/2, y, s, bold, size)
}

// wrap breaks s into lines no wider than maxW, splitting on spaces and
// falling back to a hard break for a single over-long word.
func (d *invoiceDoc) wrap(s string, maxW float64, bold bool, size float64) []string {
	var lines []string
	line := ""
	add := func(word string) {
		for d.width(word, bold, size) > maxW && len([]rune(word)) > 1 {
			r := []rune(word)
			n := len(r) - 1
			for n > 1 && d.width(string(r[:n]), bold, size) > maxW {
				n--
			}
			lines = append(lines, string(r[:n]))
			word = string(r[n:])
		}
		line = word
	}
	for _, word := range strings.Fields(s) {
		switch {
		case line == "":
			add(word)
		case d.width(line+" "+word, bold, size) <= maxW:
			line += " " + word
		default:
			lines = append(lines, line)
			add(word)
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

func (d *invoiceDoc) gray()  { d.SetTextColor(85, 85, 85) }
func (d *invoiceDoc) black() { d.SetTextColor(0, 0, 0) }

func formatTaka(v float64) string {
	s := fmt.Sprintf("%.2f", v)
	if v == float64(int64(v)) {
		s = fmt.Sprintf("%d", int64(v))
	}
	intPart, frac, _ := strings.Cut(s, ".")
	var b strings.Builder
	for i, ch := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(ch)
	}
	if frac != "" {
		b.WriteString("." + frac)
	}
	return "৳" + b.String()
}

type kvRow struct{ label, value string }

func nonEmptyRows(rows ...kvRow) []kvRow {
	var out []kvRow
	for _, r := range rows {
		if r.value != "" {
			out = append(out, r)
		}
	}
	return out
}

// kv lays out label/value rows and returns their total height. With draw
// false it only measures, so a box can be sized before it is painted.
func (d *invoiceDoc) kv(x, y, w, labelW float64, rows []kvRow, draw bool) float64 {
	const size, lineH = 8.5, 4.2
	start := y
	for _, r := range rows {
		lines := d.wrap(r.value, w-labelW, false, size)
		if draw {
			d.gray()
			d.text(x, y+3.3, strings.ToUpper(r.label), false, 6.5)
			d.black()
			for i, l := range lines {
				d.text(x+labelW, y+3.3+float64(i)*lineH, l, false, size)
			}
		}
		h := float64(len(lines)) * lineH
		if h < 4.6 {
			h = 4.6
		}
		y += h + 1.2
	}
	return y - start
}

// box paints a titled grey panel.
func (d *invoiceDoc) box(x, y, w, h float64, title string) {
	d.SetFillColor(243, 243, 243)
	d.SetDrawColor(153, 153, 153)
	d.SetLineWidth(0.2)
	d.Rect(x, y, w, h, "FD")
	d.SetFillColor(0, 0, 0)
	d.Rect(x+3, y+2.6, 0.9, 2.8, "F")
	d.black()
	d.text(x+5.5, y+5, strings.ToUpper(title), true, 7)
}

func (d *invoiceDoc) tableHeader(x, y float64, cols []float64, heads []string) {
	d.SetFillColor(0, 0, 0)
	d.Rect(x, y, innerWidth, 6.5, "F")
	d.SetTextColor(255, 255, 255)
	cx := x
	for i, h := range heads {
		if i == 0 {
			d.text(cx+3, y+4.4, strings.ToUpper(h), true, 6.5)
		} else {
			d.textRight(cx+cols[i]-3, y+4.4, strings.ToUpper(h), true, 6.5)
		}
		cx += cols[i]
	}
	d.black()
}

// stamp paints the rotated status seal: green PAID, or red UNPAID.
func (d *invoiceDoc) stamp(cx, cy float64, paid bool) {
	label, r, g, b := "P A I D", 22, 128, 60
	if !paid {
		label, r, g, b = "U N P A I D", 200, 30, 30
	}
	w := d.width(label, true, 12) + 10
	d.TransformBegin()
	d.TransformRotate(8, cx, cy)
	d.SetDrawColor(r, g, b)
	d.SetTextColor(r, g, b)
	d.SetLineWidth(0.6)
	d.Rect(cx-w/2, cy-5, w, 10, "D")
	d.textCenter(cx, cy+2.6, label, true, 12)
	d.TransformEnd()
	d.black()
}

// drawCopy paints one invoice copy starting at y and returns its bottom edge.
func (d *invoiceDoc) drawCopy(s models.InvoiceSnapshot, label string, y float64) float64 {
	x := pageMargin
	top := y
	at := s.IssuedAt.In(dhaka)
	admission := s.Kind == models.InvoiceKindAdmission

	// Header
	hdrH := 24.0
	contact := strings.Join(nonEmptyStrings(s.Issuer.Address, s.Issuer.Phone), " · ")
	if contact != "" {
		hdrH = 28
	}
	d.black()
	d.text(x+copyPad, y+11, s.Issuer.Name, true, 18)
	d.gray()
	d.text(x+copyPad, y+16, strings.ToUpper(s.Title), false, 7)
	if contact != "" {
		d.text(x+copyPad, y+20.5, contact, false, 7)
	}
	numW := d.width(s.Number, true, 8.5) + 6
	d.SetDrawColor(0, 0, 0)
	d.SetLineWidth(0.3)
	d.Rect(x+copyWidth-copyPad-numW, y+4.5, numW, 6, "D")
	d.black()
	d.textCenter(x+copyWidth-copyPad-numW/2, y+8.7, s.Number, true, 8.5)
	d.gray()
	d.textRight(x+copyWidth-copyPad, y+15, at.Format("02 Jan 2006 · 15:04"), false, 8)
	d.textRight(x+copyWidth-copyPad, y+19.5, strings.ToUpper(label), true, 6.5)
	d.SetFillColor(0, 0, 0)
	d.Rect(x, y+hdrH-1, copyWidth, 1, "F")
	y += hdrH + 5

	// Info boxes
	ix := x + copyPad
	date := at.Format("02 Jan 2006")
	if admission {
		bw := (innerWidth - 4) / 2
		st := s.Student
		student := nonEmptyRows(
			kvRow{"Name", st.Name}, kvRow{"Student ID", st.StudentID}, kvRow{"Mobile", st.Mobile},
			kvRow{"Class", st.Class}, kvRow{"Gender", st.Gender},
			kvRow{"School", strings.Join(nonEmptyStrings(st.School, st.Shift), " · ")},
			kvRow{"Guardian", guardianText(st)}, kvRow{"Address", st.Address})
		var batch []kvRow
		if b := s.Batch; b != nil {
			batch = nonEmptyRows(
				kvRow{"Batch", b.Name}, kvRow{"Code", b.Code}, kvRow{"Admitted", date}, kvRow{"Course", b.Course},
				kvRow{"Shift", b.Shift}, kvRow{"Class Days", strings.Join(b.ClassDays, ", ")},
				kvRow{"Class Time", b.ClassTime}, kvRow{"SMS No.", st.NotificationMobile})
		}
		const labelW = 20.0
		h := 8 + maxFloat(d.kv(0, 0, bw-6, labelW, student, false), d.kv(0, 0, bw-6, labelW, batch, false))
		d.box(ix, y, bw, h, "Student Information")
		d.kv(ix+3, y+8, bw-6, labelW, student, true)
		d.box(ix+bw+4, y, bw, h, "Batch Information")
		d.kv(ix+bw+7, y+8, bw-6, labelW, batch, true)
		y += h + 5
	} else {
		left := nonEmptyRows(kvRow{"Student", s.Student.Name}, kvRow{"Mobile", s.Student.Mobile}, kvRow{"Billing Month", s.Payment.BillingMonth})
		var right []kvRow
		batchName := ""
		if s.Batch != nil {
			batchName = s.Batch.Name
		}
		right = nonEmptyRows(kvRow{"Batch", batchName}, kvRow{"Method", s.Payment.Method}, kvRow{"Txn ID", s.Payment.Reference})
		colW := (innerWidth - 6) / 2
		const labelW = 24.0
		h := 8 + maxFloat(d.kv(0, 0, colW, labelW, left, false), d.kv(0, 0, colW, labelW, right, false))
		d.box(ix, y, innerWidth, h, "Student & Batch")
		d.kv(ix+3, y+8, colW, labelW, left, true)
		d.kv(ix+3+colW+6, y+8, colW, labelW, right, true)
		y += h + 5
	}

	// Fee table
	cols, heads := []float64{70, 32, 36, 36}, []string{"Description", "Fee", "Discount", "Payable"}
	if !admission {
		cols, heads = []float64{124, 50}, []string{"Description", "Amount"}
	}
	d.tableHeader(ix, y, cols, heads)
	tableTop := y
	y += 6.5
	for i, l := range s.Lines {
		rowH := 7.0
		showDiscount := admission && s.PerLineDiscount && l.Discount > 0
		if l.Note != "" || (showDiscount && l.DiscountLabel != "") {
			rowH = 10.5
		}
		if i%2 == 1 {
			d.SetFillColor(243, 243, 243)
			d.Rect(ix, y, innerWidth, rowH, "F")
		}
		d.black()
		d.text(ix+3, y+4.6, l.Label, false, 8.5)
		if l.Note != "" {
			d.gray()
			d.text(ix+3, y+8.4, l.Note, false, 6.5)
			d.black()
		}
		if admission {
			cell := func(col int, s string, bold bool) {
				off := 0.0
				for _, c := range cols[:col] {
					off += c
				}
				d.textRight(ix+off+cols[col]-3, y+4.6, s, bold, 8.5)
			}
			dash := "—"
			amount, payable := dash, dash
			if l.Amount > 0 {
				amount = formatTaka(l.Amount)
				if s.PerLineDiscount {
					payable = formatTaka(l.Total)
				}
			}
			cell(1, amount, false)
			if showDiscount {
				cell(2, "−"+formatTaka(l.Discount), false)
				if l.DiscountLabel != "" {
					d.gray()
					d.textRight(ix+cols[0]+cols[1]+cols[2]-3, y+8.4, l.DiscountLabel, false, 6.5)
					d.black()
				}
			} else {
				cell(2, dash, false)
			}
			cell(3, payable, true)
		} else {
			d.textRight(ix+cols[0]+cols[1]-3, y+4.6, formatTaka(l.Amount), true, 8.5)
		}
		y += rowH
	}
	d.SetDrawColor(0, 0, 0)
	d.SetLineWidth(0.3)
	d.Rect(ix, tableTop, innerWidth, y-tableTop, "D")
	y += 4

	if admission && s.Discount > 0 {
		d.SetFillColor(243, 243, 243)
		d.SetDrawColor(153, 153, 153)
		d.SetLineWidth(0.2)
		d.Rect(ix, y, innerWidth, 7, "FD")
		d.black()
		d.text(ix+3, y+4.6, "Discount Applied", true, 8)
		d.textRight(ix+innerWidth-3, y+4.6, fmt.Sprintf("Total saved %s (%d%% off)", formatTaka(s.Discount), s.DiscountPercent), true, 8)
		y += 11
	}

	// Payment details and totals
	const totalsW = 70.0
	leftW := innerWidth - totalsW - 8
	leftH := 0.0
	if admission {
		rows := nonEmptyRows(kvRow{"Method", s.Payment.Method}, kvRow{"Reference", s.Payment.Reference})
		leftH = d.kv(ix, y, leftW, 20, rows, true)
	}
	d.gray()
	for i, l := range d.wrap("In words: "+s.AmountInWords, leftW, false, 8) {
		d.text(ix, y+leftH+3.5+float64(i)*4, l, false, 8)
	}
	leftH += 3.5 + float64(len(d.wrap("In words: "+s.AmountInWords, leftW, false, 8)))*4

	tx := ix + innerWidth - totalsW
	ty := y
	if admission {
		d.gray()
		d.text(tx, ty+3.5, "Total Fees", false, 8)
		d.textRight(tx+totalsW, ty+3.5, formatTaka(s.Subtotal), false, 8)
		ty += 5
		if s.Discount > 0 {
			d.text(tx, ty+3.5, "Discount", false, 8)
			d.textRight(tx+totalsW, ty+3.5, "−"+formatTaka(s.Discount), false, 8)
			ty += 5
		}
	}
	d.SetFillColor(0, 0, 0)
	d.Rect(tx, ty, totalsW, 9, "F")
	d.SetTextColor(255, 255, 255)
	totalLabel := "Total Paid"
	if !s.Paid {
		totalLabel = "Total Due"
	}
	d.text(tx+3, ty+6, totalLabel, true, 10)
	d.textRight(tx+totalsW-3, ty+6, formatTaka(s.Total), true, 10)
	d.black()
	y += maxFloat(leftH, ty+9-y) + 10

	// Signatures and stamp
	d.SetDrawColor(0, 0, 0)
	d.SetLineWidth(0.3)
	d.Line(ix, y+8, ix+42, y+8)
	d.Line(ix+innerWidth-42, y+8, ix+innerWidth, y+8)
	d.gray()
	d.textCenter(ix+21, y+11.5, "Guardian / Student", false, 7)
	d.textCenter(ix+innerWidth-21, y+11.5, "Authorized Signature", false, 7)
	if s.IssuedBy != "" {
		d.textCenter(ix+innerWidth-21, y+15, s.IssuedBy, false, 6.5)
	}
	d.stamp(x+copyWidth/2, y+5, s.Paid)
	y += 20

	// Footer band and frame
	d.SetFillColor(243, 243, 243)
	d.Rect(x, y, copyWidth, 7, "F")
	d.SetDrawColor(0, 0, 0)
	d.SetLineWidth(0.3)
	d.Line(x, y, x+copyWidth, y)
	d.gray()
	d.textCenter(x+copyWidth/2, y+4.5, "Fees once paid are non-refundable. This is a computer-generated invoice. Thank you for choosing "+s.Issuer.Name+".", false, 6.5)
	y += 7
	d.Rect(x, top, copyWidth, y-top, "D")
	d.black()
	return y
}

// RenderInvoicePDF draws the snapshot as an A4 PDF. copies == 2 produces a
// Student copy and an Office copy, each on its own page to be handed out
// separately; anything else a single Original.
func RenderInvoicePDF(s models.InvoiceSnapshot, copies int) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(pageMargin, pageMargin, pageMargin)
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddUTF8FontFromBytes(familyLatin, "", fontLatin)
	pdf.AddUTF8FontFromBytes(familyLatin, "B", fontLatinBold)
	pdf.AddUTF8FontFromBytes(familyBangla, "", fontBangla)
	pdf.AddUTF8FontFromBytes(familyBangla, "B", fontBanglaBold)
	pdf.SetTitle(s.Number, true)
	pdf.SetAuthor(s.Issuer.Name, true)
	pdf.SetLineCapStyle("butt")
	pdf.AddPage()
	d := &invoiceDoc{pdf}

	if copies == 2 {
		d.drawCopy(s, "Student Copy", pageMargin)
		pdf.AddPage()
		d.drawCopy(s, "Office Copy", pageMargin)
	} else {
		d.drawCopy(s, "Original", pageMargin)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func guardianText(st models.InvoiceStudent) string {
	var parts []string
	for _, g := range [][2]string{{st.FatherName, st.FatherMobile}, {st.MotherName, st.MotherMobile}} {
		switch {
		case g[0] != "" && g[1] != "":
			parts = append(parts, fmt.Sprintf("%s (%s)", g[0], g[1]))
		case g[0] != "":
			parts = append(parts, g[0])
		}
	}
	return strings.Join(parts, " / ")
}

func nonEmptyStrings(in ...string) []string {
	var out []string
	for _, s := range in {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
