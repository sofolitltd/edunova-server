package handlers

import (
	"strings"
	"testing"
)

func TestAmountInWords(t *testing.T) {
	cases := map[float64]string{
		0:       "Zero Taka Only",
		500:     "Five Hundred Taka Only",
		1250:    "One Thousand Two Hundred Fifty Taka Only",
		100000:  "One Lakh Taka Only",
		2345678: "Twenty Three Lakh Forty Five Thousand Six Hundred Seventy Eight Taka Only",
	}
	for in, want := range cases {
		if got := amountInWords(in); got != want {
			t.Errorf("amountInWords(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestAdmissionLinesUsesStoredBreakdown(t *testing.T) {
	raw := []byte(`[{"label":"Admission Fee","amount":1000,"discount":200,"discount_label":"20% off"},{"label":"Monthly Fee","amount":800,"discount":0,"discount_label":""}]`)
	lines := admissionLines(raw, 1600, batchFees{admission: 9999})
	if len(lines) != 2 || lines[0].Total != 800 || lines[1].Note != "per month" {
		t.Fatalf("unexpected lines: %+v", lines)
	}
}

func TestAdmissionLinesFallback(t *testing.T) {
	fees := batchFees{admission: 1000, note: 500, monthly: 800}

	lines := admissionLines(nil, 2300, fees)
	if len(lines) != 3 || lines[2].Total != 800 {
		t.Fatalf("expected the three batch fee lines, got %+v", lines)
	}

	// Paid more than the batch's current fees: the breakdown is unknowable.
	lines = admissionLines([]byte("null"), 5000, fees)
	if len(lines) != 1 || lines[0].Label != "Enrollment Fee" || lines[0].Amount != 5000 {
		t.Fatalf("expected a single Enrollment Fee line, got %+v", lines)
	}
}

func TestClassSchedule(t *testing.T) {
	if got := clockRange("15:30", "17:00"); got != "3:30 PM – 5:00 PM" {
		t.Errorf("clockRange = %q", got)
	}
	if got := clockLabel("00:05"); got != "12:05 AM" {
		t.Errorf("clockLabel = %q", got)
	}
	got := strings.Join(sortedDays([]string{"Mon", "Sat", "Mon", "Wed"}), ",")
	if got != "Sat,Mon,Wed" {
		t.Errorf("sortedDays = %q", got)
	}
}
