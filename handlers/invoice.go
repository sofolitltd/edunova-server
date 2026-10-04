package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"edunova-server/config"
	"edunova-server/database"
	"edunova-server/models"
)

// Invoices are issued once, when the money event happens (an approved batch
// enrollment, a verified payment), and stored as an immutable snapshot. Every
// client and the PDF render that snapshot; nothing recomputes from live data.
// Rows that predate this ledger are snapshotted on first request or by
// BackfillInvoices, using the data as it is then.

var (
	errInvoiceNotFound      = errors.New("invoice source not found")
	errInvoiceNotApplicable = errors.New("invoice is not available yet")
)

type invoiceDraft struct {
	snapshot     models.InvoiceSnapshot
	enrollmentID *int
	paymentID    *int
	userID       *int
	batchID      *int
}

type invoiceRecord struct {
	models.Invoice
	userID *int
}

func (r *invoiceRecord) ownedBy(userID int, mobile string) bool {
	if r.userID != nil {
		return *r.userID == userID
	}
	return r.Student.Mobile != "" && r.Student.Mobile == mobile
}

type batchFees struct{ admission, note, monthly int }

func invoiceIssuer() models.InvoiceIssuer {
	return models.InvoiceIssuer{
		Name:    config.AppConfig.InvoiceIssuerName,
		Address: config.AppConfig.InvoiceIssuerAddress,
		Phone:   config.AppConfig.InvoiceIssuerPhone,
	}
}

// ---------- snapshot builders ----------

func buildAdmissionDraft(ctx context.Context, enrollmentID int) (*invoiceDraft, error) {
	var (
		fullName, mobile, studentID, method, reference, status, enrolledBy, courseName string
		userID, batchID                                                                *int
		amount                                                                         int
		feeJSON                                                                        []byte
		createdAt                                                                      time.Time
	)
	err := database.DB.QueryRow(ctx,
		`SELECT e.full_name, e.mobile, COALESCE(e.student_id,''), e.user_id, e.batch_id, e.amount,
		        COALESCE(e.payment_method,''), COALESCE(e.sent_from,''), e.status, COALESCE(e.enrolled_by,''),
		        e.fee_breakdown, e.created_at, COALESCE(c.title,'')
		 FROM enrollments e LEFT JOIN courses c ON c.id = e.course_id
		 WHERE e.id = $1`, enrollmentID,
	).Scan(&fullName, &mobile, &studentID, &userID, &batchID, &amount, &method, &reference,
		&status, &enrolledBy, &feeJSON, &createdAt, &courseName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errInvoiceNotFound
	}
	if err != nil {
		return nil, err
	}
	if status != "approved" || batchID == nil {
		return nil, errInvoiceNotApplicable
	}

	batch, fees, err := loadInvoiceBatch(ctx, *batchID, courseName)
	if err != nil {
		return nil, err
	}
	student, resolvedUserID := loadInvoiceStudent(ctx, userID, mobile)
	student.Name = fullName
	if studentID != "" {
		student.StudentID = studentID
	}

	paid := float64(amount)
	lines := admissionLines(feeJSON, paid, fees)
	subtotal, lineDiscount := 0.0, 0.0
	for _, l := range lines {
		subtotal += l.Amount
		lineDiscount += l.Discount
	}
	discount := math.Max(0, subtotal-paid)
	percent := 0
	if subtotal > 0 && discount > 0 {
		percent = int(math.Round(discount / subtotal * 100))
	}

	return &invoiceDraft{
		snapshot: models.InvoiceSnapshot{
			Kind:            models.InvoiceKindAdmission,
			Number:          fmt.Sprintf("INV-%05d", enrollmentID),
			Title:           "Admission Invoice",
			IssuedAt:        createdAt,
			Issuer:          invoiceIssuer(),
			Student:         student,
			Batch:           batch,
			Lines:           lines,
			PerLineDiscount: lineDiscount == discount,
			Subtotal:        subtotal,
			Discount:        discount,
			DiscountPercent: percent,
			Total:           paid,
			AmountInWords:   amountInWords(paid),
			Payment:         models.InvoicePayment{Method: methodLabel(method), Reference: reference},
			IssuedBy:        enrolledBy,
			Paid:            true,
		},
		enrollmentID: &enrollmentID,
		userID:       resolvedUserID,
		batchID:      batchID,
	}, nil
}

func buildPaymentDraft(ctx context.Context, paymentID int) (*invoiceDraft, error) {
	var (
		userID                                                     int
		batchID                                                    *int
		amount                                                     float64
		method, txnID, status, receiptNo, month                    string
		courseName, batchName, verifiedByName, mobile, studentName string
		year                                                       int
		createdAt                                                  time.Time
	)
	err := database.DB.QueryRow(ctx,
		`SELECT p.user_id, p.batch_id, p.amount, p.method, COALESCE(p.transaction_id,''), p.status,
		        COALESCE(p.receipt_number,''), COALESCE(p.month,''), COALESCE(p.year,0), p.created_at,
		        COALESCE(c.title,''), COALESCE(b.name,''), COALESCE(au.full_name,''), u.mobile, u.full_name
		 FROM payments p
		 JOIN users u ON u.id = p.user_id
		 LEFT JOIN courses c ON c.id = p.course_id
		 LEFT JOIN batches b ON b.id = p.batch_id
		 LEFT JOIN admin_users au ON au.id = p.verified_by
		 WHERE p.id = $1`, paymentID,
	).Scan(&userID, &batchID, &amount, &method, &txnID, &status, &receiptNo, &month, &year, &createdAt,
		&courseName, &batchName, &verifiedByName, &mobile, &studentName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errInvoiceNotFound
	}
	if err != nil {
		return nil, err
	}
	if status != "verified" {
		return nil, errInvoiceNotApplicable
	}

	student, _ := loadInvoiceStudent(ctx, &userID, mobile)
	student.Name = studentName

	var batch *models.InvoiceBatch
	if batchID != nil {
		b, _, err := loadInvoiceBatch(ctx, *batchID, courseName)
		if err != nil {
			return nil, err
		}
		batch = b
		_ = database.DB.QueryRow(ctx,
			`SELECT COALESCE(student_id,'') FROM enrollments
			 WHERE user_id = $1 AND batch_id = $2 AND status = 'approved' ORDER BY id DESC LIMIT 1`,
			userID, *batchID).Scan(&student.StudentID)
	} else if courseName != "" {
		batch = &models.InvoiceBatch{Name: courseName}
	}

	period := month
	if month != "" && year > 0 {
		period = fmt.Sprintf("%s %d", month, year)
	}
	title, label := "Payment Receipt", courseName
	if month != "" {
		title, label = "Monthly Fee Receipt", "Monthly Fee — "+period
	}
	if label == "" {
		label = "Payment"
	}
	number := receiptNo
	if number == "" {
		number = fmt.Sprintf("RCPT-%05d", paymentID)
	}

	return &invoiceDraft{
		snapshot: models.InvoiceSnapshot{
			Kind:            models.InvoiceKindPayment,
			Number:          number,
			Title:           title,
			IssuedAt:        createdAt,
			Issuer:          invoiceIssuer(),
			Student:         student,
			Batch:           batch,
			Lines:           []models.InvoiceLine{{Label: label, Amount: amount, Total: amount}},
			PerLineDiscount: true,
			Subtotal:        amount,
			Total:           amount,
			AmountInWords:   amountInWords(amount),
			Payment:         models.InvoicePayment{Method: methodLabel(method), Reference: txnID, BillingMonth: period},
			IssuedBy:        verifiedByName,
			Paid:            true,
		},
		paymentID: &paymentID,
		userID:    &userID,
		batchID:   batchID,
	}, nil
}

// loadInvoiceStudent reads the student's profile. Enrollments made through the
// public flow carry no user_id, so it falls back to matching on mobile.
func loadInvoiceStudent(ctx context.Context, userID *int, mobile string) (models.InvoiceStudent, *int) {
	st := models.InvoiceStudent{Mobile: mobile}
	q := `SELECT id, full_name, mobile, COALESCE(gender,''), COALESCE(student_class,''), COALESCE(school,''),
	             COALESCE(shift,''), COALESCE(father_name,''), COALESCE(father_mobile,''), COALESCE(mother_name,''),
	             COALESCE(mother_mobile,''), COALESCE(notification_mobile,''),
	             COALESCE(NULLIF(present_address,''), address, '')
	      FROM users WHERE `
	var row pgx.Row
	if userID != nil {
		row = database.DB.QueryRow(ctx, q+`id = $1`, *userID)
	} else {
		row = database.DB.QueryRow(ctx, q+`mobile = $1`, mobile)
	}
	var uid int
	if err := row.Scan(&uid, &st.Name, &st.Mobile, &st.Gender, &st.Class, &st.School, &st.Shift,
		&st.FatherName, &st.FatherMobile, &st.MotherName, &st.MotherMobile, &st.NotificationMobile, &st.Address); err != nil {
		return st, userID
	}
	return st, &uid
}

func loadInvoiceBatch(ctx context.Context, batchID int, courseName string) (*models.InvoiceBatch, batchFees, error) {
	var (
		b           models.InvoiceBatch
		fees        batchFees
		days        []string
		start, end  string
		batchCourse string
	)
	err := database.DB.QueryRow(ctx,
		`SELECT b.name, COALESCE(b.code,''), COALESCE(b.shift,''), COALESCE(c.title,''), COALESCE(b.days,'{}'),
		        COALESCE(b.start_time,''), COALESCE(b.end_time,''),
		        COALESCE(b.admission_fee,0), COALESCE(b.note_fee,0), COALESCE(b.monthly_fee,0)
		 FROM batches b LEFT JOIN courses c ON c.id = b.course_id WHERE b.id = $1`, batchID,
	).Scan(&b.Name, &b.Code, &b.Shift, &batchCourse, &days, &start, &end, &fees.admission, &fees.note, &fees.monthly)
	if err != nil {
		return nil, fees, err
	}
	b.Course = courseName
	if b.Course == "" {
		b.Course = batchCourse
	}

	var routineDays []string
	var routineTimes []string
	if rows, err := database.DB.Query(ctx,
		`SELECT COALESCE(days,'{}'), COALESCE(start_time,''), COALESCE(end_time,'') FROM batch_subjects WHERE batch_id = $1`, batchID); err == nil {
		defer rows.Close()
		for rows.Next() {
			var d []string
			var s, e string
			if rows.Scan(&d, &s, &e) == nil {
				routineDays = append(routineDays, d...)
				if s != "" {
					routineTimes = append(routineTimes, clockRange(s, e))
				}
			}
		}
	}

	if len(days) == 0 {
		days = routineDays
	}
	b.ClassDays = sortedDays(days)
	if start != "" {
		b.ClassTime = clockRange(start, end)
	} else {
		b.ClassTime = strings.Join(uniqueStrings(routineTimes), " / ")
	}
	return &b, fees, nil
}

type feeBreakdownItem struct {
	Label         string  `json:"label"`
	Amount        float64 `json:"amount"`
	Discount      float64 `json:"discount"`
	DiscountLabel string  `json:"discount_label"`
}

// admissionLines uses the fee breakdown stored on the enrollment. Enrollments
// from before it was stored fall back to the batch's current fees, so their
// per-line discount split is unknown.
func admissionLines(feeJSON []byte, paid float64, fees batchFees) []models.InvoiceLine {
	var items []feeBreakdownItem
	if len(feeJSON) > 0 {
		_ = json.Unmarshal(feeJSON, &items)
	}
	if len(items) > 0 {
		lines := make([]models.InvoiceLine, 0, len(items))
		for _, f := range items {
			note := ""
			if f.Label == "Monthly Fee" {
				note = "per month"
			}
			lines = append(lines, models.InvoiceLine{
				Label: f.Label, Note: note, Amount: f.Amount, Discount: f.Discount,
				DiscountLabel: f.DiscountLabel, Total: math.Max(0, f.Amount-f.Discount),
			})
		}
		return lines
	}

	fallback := []models.InvoiceLine{
		{Label: "Admission Fee", Amount: float64(fees.admission)},
		{Label: "Note Fee", Amount: float64(fees.note)},
		{Label: "Monthly Fee", Note: "per month", Amount: float64(fees.monthly)},
	}
	total := 0.0
	for _, l := range fallback {
		total += l.Amount
	}
	if total < paid || total == 0 {
		return []models.InvoiceLine{{Label: "Enrollment Fee", Amount: paid, Total: paid}}
	}
	for i := range fallback {
		fallback[i].Total = fallback[i].Amount
	}
	return fallback
}

// ---------- storage ----------

const invoiceSelect = `SELECT id, enrollment_id, payment_id, user_id, snapshot FROM invoices`

func scanInvoice(row pgx.Row) (*invoiceRecord, error) {
	var rec invoiceRecord
	var raw []byte
	if err := row.Scan(&rec.ID, &rec.EnrollmentID, &rec.PaymentID, &rec.userID, &raw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errInvoiceNotFound
		}
		return nil, err
	}
	if err := json.Unmarshal(raw, &rec.InvoiceSnapshot); err != nil {
		return nil, err
	}
	return &rec, nil
}

func findInvoice(ctx context.Context, where string, arg any) (*invoiceRecord, error) {
	return scanInvoice(database.DB.QueryRow(ctx, invoiceSelect+` WHERE `+where, arg))
}

func storeInvoice(ctx context.Context, d *invoiceDraft, lookupWhere string, lookupArg any) (*invoiceRecord, error) {
	raw, err := json.Marshal(d.snapshot)
	if err != nil {
		return nil, err
	}
	var id int
	err = database.DB.QueryRow(ctx,
		`INSERT INTO invoices (kind, number, enrollment_id, payment_id, user_id, batch_id, total, snapshot, issued_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9)
		 ON CONFLICT DO NOTHING
		 RETURNING id`,
		d.snapshot.Kind, d.snapshot.Number, d.enrollmentID, d.paymentID, d.userID, d.batchID,
		d.snapshot.Total, string(raw), d.snapshot.IssuedAt,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		// A concurrent request issued it first.
		return findInvoice(ctx, lookupWhere, lookupArg)
	}
	if err != nil {
		return nil, err
	}
	return findInvoice(ctx, `id = $1`, id)
}

func ensureAdmissionInvoice(ctx context.Context, enrollmentID int) (*invoiceRecord, error) {
	const where = `kind = 'admission' AND enrollment_id = $1`
	if rec, err := findInvoice(ctx, where, enrollmentID); !errors.Is(err, errInvoiceNotFound) {
		return rec, err
	}
	draft, err := buildAdmissionDraft(ctx, enrollmentID)
	if err != nil {
		return nil, err
	}
	return storeInvoice(ctx, draft, where, enrollmentID)
}

func ensurePaymentInvoice(ctx context.Context, paymentID int) (*invoiceRecord, error) {
	const where = `kind = 'payment' AND payment_id = $1`
	if rec, err := findInvoice(ctx, where, paymentID); !errors.Is(err, errInvoiceNotFound) {
		return rec, err
	}
	draft, err := buildPaymentDraft(ctx, paymentID)
	if err != nil {
		return nil, err
	}
	return storeInvoice(ctx, draft, where, paymentID)
}

// issueAdmissionInvoice and issuePaymentInvoice are the eager hooks called
// when the money event happens. A failure must not fail the business action,
// so it is logged; the invoice is then issued on first request or backfill.
func issueAdmissionInvoice(ctx context.Context, enrollmentID int) {
	if _, err := ensureAdmissionInvoice(ctx, enrollmentID); err != nil && !errors.Is(err, errInvoiceNotApplicable) {
		log.Printf("invoice: admission invoice for enrollment %d: %v", enrollmentID, err)
	}
}

func issuePaymentInvoice(ctx context.Context, paymentID int) {
	if _, err := ensurePaymentInvoice(ctx, paymentID); err != nil && !errors.Is(err, errInvoiceNotApplicable) {
		log.Printf("invoice: payment invoice for payment %d: %v", paymentID, err)
	}
}

// BackfillInvoices issues invoices for approved batch enrollments and verified
// payments that have none yet. Safe to run on every startup.
func BackfillInvoices(ctx context.Context) {
	issued := 0
	for _, src := range []struct {
		query string
		issue func(context.Context, int)
	}{
		{`SELECT e.id FROM enrollments e WHERE e.status = 'approved' AND e.batch_id IS NOT NULL
		  AND NOT EXISTS (SELECT 1 FROM invoices i WHERE i.kind = 'admission' AND i.enrollment_id = e.id) ORDER BY e.id`, issueAdmissionInvoice},
		{`SELECT p.id FROM payments p WHERE p.status = 'verified'
		  AND NOT EXISTS (SELECT 1 FROM invoices i WHERE i.kind = 'payment' AND i.payment_id = p.id) ORDER BY p.id`, issuePaymentInvoice},
	} {
		rows, err := database.DB.Query(ctx, src.query)
		if err != nil {
			log.Printf("invoice backfill: %v", err)
			continue
		}
		var ids []int
		for rows.Next() {
			var id int
			if rows.Scan(&id) == nil {
				ids = append(ids, id)
			}
		}
		rows.Close()
		for _, id := range ids {
			src.issue(ctx, id)
			issued++
		}
	}
	if issued > 0 {
		log.Printf("invoice backfill: processed %d missing invoices", issued)
	}
}

// ---------- HTTP ----------

func invoiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errInvoiceNotFound):
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "invoice not found"})
	case errors.Is(err, errInvoiceNotApplicable):
		c.JSON(http.StatusConflict, models.ErrorResponse{Error: err.Error()})
	default:
		log.Printf("invoice: %v", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
	}
}

func pathID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return 0, false
	}
	return id, true
}

func respondInvoice(c *gin.Context, rec *invoiceRecord, err error) {
	if err != nil {
		invoiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, rec.Invoice)
}

func currentUser(c *gin.Context) (int, string, bool) {
	userID, ok := c.Get("user_id")
	id, isInt := userID.(int)
	if !ok || !isInt {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "user not found"})
		return 0, "", false
	}
	return id, c.GetString("mobile"), true
}

// Student endpoints. Ownership is checked against the source row before an
// invoice is issued, so other students' ids reveal nothing and cost no writes.

func UserGetPaymentInvoice(c *gin.Context) {
	userID, _, ok := currentUser(c)
	paymentID, idOK := pathID(c)
	if !ok || !idOK {
		return
	}
	ctx := c.Request.Context()
	var owner int
	if err := database.DB.QueryRow(ctx, `SELECT user_id FROM payments WHERE id = $1`, paymentID).Scan(&owner); err != nil || owner != userID {
		invoiceError(c, errInvoiceNotFound)
		return
	}
	rec, err := ensurePaymentInvoice(ctx, paymentID)
	respondInvoice(c, rec, err)
}

func UserGetEnrollmentInvoice(c *gin.Context) {
	userID, mobile, ok := currentUser(c)
	enrollmentID, idOK := pathID(c)
	if !ok || !idOK {
		return
	}
	ctx := c.Request.Context()
	var ownerID *int
	var ownerMobile string
	if err := database.DB.QueryRow(ctx, `SELECT user_id, mobile FROM enrollments WHERE id = $1`, enrollmentID).Scan(&ownerID, &ownerMobile); err != nil {
		invoiceError(c, errInvoiceNotFound)
		return
	}
	if !((ownerID != nil && *ownerID == userID) || ownerMobile == mobile) {
		invoiceError(c, errInvoiceNotFound)
		return
	}
	rec, err := ensureAdmissionInvoice(ctx, enrollmentID)
	respondInvoice(c, rec, err)
}

func UserGetInvoice(c *gin.Context) {
	rec, ok := userInvoice(c)
	if ok {
		c.JSON(http.StatusOK, rec.Invoice)
	}
}

func UserGetInvoicePDF(c *gin.Context) {
	if rec, ok := userInvoice(c); ok {
		serveInvoicePDF(c, rec)
	}
}

func userInvoice(c *gin.Context) (*invoiceRecord, bool) {
	userID, mobile, ok := currentUser(c)
	id, idOK := pathID(c)
	if !ok || !idOK {
		return nil, false
	}
	rec, err := findInvoice(c.Request.Context(), `id = $1`, id)
	if err == nil && !rec.ownedBy(userID, mobile) {
		err = errInvoiceNotFound
	}
	if err != nil {
		invoiceError(c, err)
		return nil, false
	}
	return rec, true
}

// Admin endpoints.

func AdminGetPaymentInvoice(c *gin.Context) {
	if id, ok := pathID(c); ok {
		rec, err := ensurePaymentInvoice(c.Request.Context(), id)
		respondInvoice(c, rec, err)
	}
}

func AdminGetEnrollmentInvoice(c *gin.Context) {
	if id, ok := pathID(c); ok {
		rec, err := ensureAdmissionInvoice(c.Request.Context(), id)
		respondInvoice(c, rec, err)
	}
}

func AdminGetInvoice(c *gin.Context) {
	if id, ok := pathID(c); ok {
		rec, err := findInvoice(c.Request.Context(), `id = $1`, id)
		respondInvoice(c, rec, err)
	}
}

func AdminGetInvoicePDF(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	rec, err := findInvoice(c.Request.Context(), `id = $1`, id)
	if err != nil {
		invoiceError(c, err)
		return
	}
	serveInvoicePDF(c, rec)
}

// ---------- formatting helpers ----------

var (
	invoiceOnes = []string{"", "One", "Two", "Three", "Four", "Five", "Six", "Seven", "Eight", "Nine", "Ten", "Eleven",
		"Twelve", "Thirteen", "Fourteen", "Fifteen", "Sixteen", "Seventeen", "Eighteen", "Nineteen"}
	invoiceTens = []string{"", "", "Twenty", "Thirty", "Forty", "Fifty", "Sixty", "Seventy", "Eighty", "Ninety"}
	weekOrder   = []string{"sat", "sun", "mon", "tue", "wed", "thu", "fri"}
)

func wordsBelowThousand(n int) string {
	hundreds := ""
	if n >= 100 {
		hundreds = invoiceOnes[n/100] + " Hundred "
	}
	rest := n % 100
	tail := ""
	if rest < 20 {
		tail = invoiceOnes[rest]
	} else {
		tail = invoiceTens[rest/10] + " " + invoiceOnes[rest%10]
	}
	return strings.TrimSpace(hundreds + tail)
}

// amountInWords spells out the whole-taka part in the South Asian grouping.
func amountInWords(v float64) string {
	n := int(math.Floor(v))
	if n <= 0 {
		return "Zero Taka Only"
	}
	var words []string
	for _, p := range []struct {
		v    int
		unit string
	}{{n / 10000000, "Crore"}, {n / 100000 % 100, "Lakh"}, {n / 1000 % 100, "Thousand"}} {
		if p.v > 0 {
			words = append(words, wordsBelowThousand(p.v)+" "+p.unit)
		}
	}
	if n%1000 > 0 {
		words = append(words, wordsBelowThousand(n%1000))
	}
	return strings.Join(words, " ") + " Taka Only"
}

func clockLabel(t string) string {
	parts := strings.SplitN(t, ":", 2)
	h, err := strconv.Atoi(parts[0])
	if err != nil {
		return t
	}
	m := 0
	if len(parts) == 2 {
		m, _ = strconv.Atoi(parts[1])
	}
	suffix := "AM"
	if h >= 12 {
		suffix = "PM"
	}
	if h12 := h % 12; h12 != 0 {
		return fmt.Sprintf("%d:%02d %s", h12, m, suffix)
	}
	return fmt.Sprintf("12:%02d %s", m, suffix)
}

func clockRange(start, end string) string {
	var parts []string
	for _, t := range []string{start, end} {
		if t != "" {
			parts = append(parts, clockLabel(t))
		}
	}
	return strings.Join(parts, " – ")
}

func weekIndex(day string) int {
	short := strings.ToLower(day)
	if len(short) > 3 {
		short = short[:3]
	}
	for i, d := range weekOrder {
		if d == short {
			return i
		}
	}
	return -1
}

func sortedDays(days []string) []string {
	out := uniqueStrings(days)
	sort.SliceStable(out, func(i, j int) bool { return weekIndex(out[i]) < weekIndex(out[j]) })
	return out
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	var out []string
	for _, s := range in {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func methodLabel(m string) string {
	switch strings.ToLower(m) {
	case "", "cash":
		return "Cash"
	case "online":
		return "Online"
	case "bkash":
		return "bKash"
	case "nagad":
		return "Nagad"
	}
	return strings.ToUpper(m[:1]) + m[1:]
}
