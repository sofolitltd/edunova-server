package models

import "time"

const (
	InvoiceKindAdmission = "admission"
	InvoiceKindPayment   = "payment"
)

type InvoiceIssuer struct {
	Name    string `json:"name"`
	Address string `json:"address,omitempty"`
	Phone   string `json:"phone,omitempty"`
}

type InvoiceStudent struct {
	Name               string `json:"name"`
	StudentID          string `json:"student_id,omitempty"`
	Mobile             string `json:"mobile"`
	Gender             string `json:"gender,omitempty"`
	Class              string `json:"class,omitempty"`
	School             string `json:"school,omitempty"`
	Shift              string `json:"shift,omitempty"`
	FatherName         string `json:"father_name,omitempty"`
	FatherMobile       string `json:"father_mobile,omitempty"`
	MotherName         string `json:"mother_name,omitempty"`
	MotherMobile       string `json:"mother_mobile,omitempty"`
	NotificationMobile string `json:"notification_mobile,omitempty"`
	Address            string `json:"address,omitempty"`
}

type InvoiceBatch struct {
	Name      string   `json:"name"`
	Code      string   `json:"code,omitempty"`
	Course    string   `json:"course,omitempty"`
	Shift     string   `json:"shift,omitempty"`
	ClassDays []string `json:"class_days,omitempty"`
	ClassTime string   `json:"class_time,omitempty"`
}

type InvoiceLine struct {
	Label         string  `json:"label"`
	Note          string  `json:"note,omitempty"`
	Amount        float64 `json:"amount"`
	Discount      float64 `json:"discount"`
	DiscountLabel string  `json:"discount_label,omitempty"`
	Total         float64 `json:"total"`
}

type InvoicePayment struct {
	Method       string `json:"method,omitempty"`
	Reference    string `json:"reference,omitempty"`
	BillingMonth string `json:"billing_month,omitempty"`
}

// InvoiceSnapshot is everything an invoice shows, frozen at issue time. It is
// the single source of truth for every client and for the PDF, so none of them
// recompute totals or re-read live fee/profile data.
type InvoiceSnapshot struct {
	Kind            string         `json:"kind"`
	Number          string         `json:"number"`
	Title           string         `json:"title"`
	IssuedAt        time.Time      `json:"issued_at"`
	Issuer          InvoiceIssuer  `json:"issuer"`
	Student         InvoiceStudent `json:"student"`
	Batch           *InvoiceBatch  `json:"batch,omitempty"`
	Lines           []InvoiceLine  `json:"lines"`
	PerLineDiscount bool           `json:"per_line_discount"`
	Subtotal        float64        `json:"subtotal"`
	Discount        float64        `json:"discount"`
	DiscountPercent int            `json:"discount_percent"`
	Total           float64        `json:"total"`
	AmountInWords   string         `json:"amount_in_words"`
	Payment         InvoicePayment `json:"payment"`
	IssuedBy        string         `json:"issued_by,omitempty"`
	Paid            bool           `json:"paid"`
}

type Invoice struct {
	ID           int  `json:"id"`
	EnrollmentID *int `json:"enrollment_id"`
	PaymentID    *int `json:"payment_id"`
	InvoiceSnapshot
}
