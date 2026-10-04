package services

import (
	"bytes"
	"os"
	"testing"
	"time"

	"edunova-server/models"
)

func sampleSnapshot(kind string) models.InvoiceSnapshot {
	s := models.InvoiceSnapshot{
		Kind:     kind,
		Number:   "INV-00012",
		Title:    "Admission Invoice",
		IssuedAt: time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC),
		Issuer:   models.InvoiceIssuer{Name: "EduNova", Address: "Dhaka, Bangladesh", Phone: "01700000000"},
		Student: models.InvoiceStudent{
			Name: "রহিম উদ্দিন", StudentID: "C9A0001", Mobile: "01711111111", Class: "Nine", Gender: "Male",
			School: "Dhaka Residential Model College", Shift: "Morning",
			FatherName: "করিম উদ্দিন", FatherMobile: "01722222222", Address: "বাড়ি ১২, রোড ৫, ধানমন্ডি, ঢাকা",
			NotificationMobile: "01722222222",
		},
		Batch: &models.InvoiceBatch{Name: "Class 9 - A", Code: "C9A", Course: "SSC Math", Shift: "Morning",
			ClassDays: []string{"Sat", "Mon", "Wed"}, ClassTime: "3:00 PM – 4:30 PM"},
		Lines: []models.InvoiceLine{
			{Label: "Admission Fee", Amount: 1000, Discount: 200, DiscountLabel: "20% off", Total: 800},
			{Label: "Note Fee", Amount: 500, Total: 500},
			{Label: "Monthly Fee", Note: "per month", Amount: 800, Total: 800},
		},
		PerLineDiscount: true,
		Subtotal:        2300,
		Discount:        200,
		DiscountPercent: 9,
		Total:           2100,
		AmountInWords:   "Two Thousand One Hundred Taka Only",
		Payment:         models.InvoicePayment{Method: "Cash", Reference: "REF-1"},
		IssuedBy:        "staff",
		Paid:            true,
	}
	if kind == models.InvoiceKindPayment {
		s.Number, s.Title = "EDU-12345-6789", "Monthly Fee Receipt"
		s.Lines = []models.InvoiceLine{{Label: "Monthly Fee — October 2026", Amount: 800, Total: 800}}
		s.Subtotal, s.Discount, s.Total = 800, 0, 800
		s.Payment = models.InvoicePayment{Method: "bKash", Reference: "TXN123456", BillingMonth: "October 2026"}
	}
	return s
}

func TestRenderInvoicePDF(t *testing.T) {
	for _, kind := range []string{models.InvoiceKindAdmission, models.InvoiceKindPayment} {
		for _, copies := range []int{1, 2} {
			out, err := RenderInvoicePDF(sampleSnapshot(kind), copies)
			if err != nil {
				t.Fatalf("%s/%d copies: %v", kind, copies, err)
			}
			if !bytes.HasPrefix(out, []byte("%PDF-")) || len(out) < 2000 {
				t.Errorf("%s/%d copies: not a plausible PDF (%d bytes)", kind, copies, len(out))
			}
			// Set INVOICE_PDF_DIR to look at the output.
			if dir := os.Getenv("INVOICE_PDF_DIR"); dir != "" {
				name := dir + "/" + kind + map[int]string{1: "-single", 2: "-double"}[copies] + ".pdf"
				if err := os.WriteFile(name, out, 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

func TestRenderUnpaidInvoicePDF(t *testing.T) {
	snap := sampleSnapshot(models.InvoiceKindPayment)
	snap.Paid = false
	out, err := RenderInvoicePDF(snap, 1)
	if err != nil || !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatalf("unpaid render failed: %v", err)
	}
	if dir := os.Getenv("INVOICE_PDF_DIR"); dir != "" {
		_ = os.WriteFile(dir+"/payment-unpaid.pdf", out, 0o644)
	}
}

func TestSplitRunsSeparatesBangla(t *testing.T) {
	runs := splitRuns("Name রহিম 12")
	if len(runs) != 3 || runs[0].family != familyLatin || runs[1].family != familyBangla || runs[2].family != familyLatin {
		t.Fatalf("unexpected runs: %+v", runs)
	}
}

func TestFormatTaka(t *testing.T) {
	cases := map[float64]string{0: "৳0", 800: "৳800", 1234567: "৳1,234,567", 99.5: "৳99.50"}
	for in, want := range cases {
		if got := formatTaka(in); got != want {
			t.Errorf("formatTaka(%v) = %q, want %q", in, got, want)
		}
	}
}
