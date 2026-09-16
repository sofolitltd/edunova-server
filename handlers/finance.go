package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"edunova-server/database"
	"edunova-server/models"
)

func AdminGetFinanceStats(c *gin.Context) {
	ctx := context.Background()
	var stats models.FinanceStats

	// Revenue from approved enrollments
	_ = database.DB.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM enrollments WHERE status = 'approved'`,
	).Scan(&stats.TotalRevenue)

	// Pending payments
	_ = database.DB.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM enrollments WHERE status = 'pending'`,
	).Scan(&stats.PendingPayments)

	// Approved payments count
	_ = database.DB.QueryRow(ctx,
		`SELECT COUNT(*) FROM enrollments WHERE status = 'approved'`,
	).Scan(&stats.ApprovedPayments)

	// Total expenses
	_ = database.DB.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM expenses`,
	).Scan(&stats.TotalExpenses)

	stats.NetProfit = stats.TotalRevenue - stats.TotalExpenses

	// Monthly data (last 6 months)
	rows, err := database.DB.Query(ctx,
		`SELECT to_char(month, 'YYYY-MM') as m,
			COALESCE(r.total, 0) as revenue,
			COALESCE(e.total, 0) as expenses
		 FROM generate_series(date_trunc('month', NOW() - interval '5 months'), date_trunc('month', NOW()), interval '1 month') as month
		 LEFT JOIN (
			SELECT date_trunc('month', created_at) as m, SUM(amount) as total
			FROM enrollments WHERE status = 'approved'
			GROUP BY 1
		 ) r ON r.m = month
		 LEFT JOIN (
			SELECT date_trunc('month', date) as m, SUM(amount) as total
			FROM expenses
			GROUP BY 1
		 ) e ON e.m = month
		 ORDER BY month`,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var mf models.MonthlyFinance
			if err := rows.Scan(&mf.Month, &mf.Revenue, &mf.Expenses); err == nil {
				stats.MonthlyData = append(stats.MonthlyData, mf)
			}
		}
	}
	if stats.MonthlyData == nil {
		stats.MonthlyData = []models.MonthlyFinance{}
	}

	// Recent enrollments (last 5)
	erows, err := database.DB.Query(ctx,
		`SELECT e.id, COALESCE(e.course_id, 0), COALESCE(c.title,''), e.full_name, e.mobile, e.payment_method,
			e.mobile_banking, e.amount, e.sent_from, e.sent_to, e.referral_source,
			e.status, e.created_at, e.updated_at
		 FROM enrollments e
		 LEFT JOIN courses c ON e.course_id = c.id
		 ORDER BY e.created_at DESC LIMIT 5`,
	)
	if err == nil {
		defer erows.Close()
		for erows.Next() {
			var en models.Enrollment
			if err := erows.Scan(&en.ID, &en.CourseID, &en.CourseName, &en.FullName, &en.Mobile,
				&en.PaymentMethod, &en.MobileBanking, &en.Amount, &en.SentFrom, &en.SentTo,
				&en.ReferralSource, &en.Status, &en.CreatedAt, &en.UpdatedAt); err == nil {
				stats.RecentEnrollments = append(stats.RecentEnrollments, en)
			}
		}
	}
	if stats.RecentEnrollments == nil {
		stats.RecentEnrollments = []models.Enrollment{}
	}

	// Recent expenses (last 5)
	xrows, err := database.DB.Query(ctx,
		`SELECT id, category, description, amount, COALESCE(date::text, ''), COALESCE(notes, ''), COALESCE(created_by, 0), created_at
		 FROM expenses ORDER BY created_at DESC LIMIT 5`,
	)
	if err == nil {
		defer xrows.Close()
		for xrows.Next() {
			var ex models.Expense
			if err := xrows.Scan(&ex.ID, &ex.Category, &ex.Description, &ex.Amount, &ex.Date, &ex.Notes, &ex.CreatedBy, &ex.CreatedAt); err == nil {
				stats.RecentExpenses = append(stats.RecentExpenses, ex)
			}
		}
	}
	if stats.RecentExpenses == nil {
		stats.RecentExpenses = []models.Expense{}
	}

	c.JSON(http.StatusOK, stats)
}

func AdminGetBatchFinance(c *gin.Context) {
	batchID := c.Param("id")
	ctx := context.Background()
	var stats models.BatchFinanceStats

	_ = database.DB.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0), COUNT(*) FROM enrollments WHERE batch_id = $1 AND status = 'approved'`,
		batchID,
	).Scan(&stats.TotalRevenue, &stats.ApprovedCount)

	_ = database.DB.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0), COUNT(*) FROM enrollments WHERE batch_id = $1 AND status = 'pending'`,
		batchID,
	).Scan(&stats.PendingPayments, &stats.PendingCount)

	rows, err := database.DB.Query(ctx,
		`SELECT to_char(month, 'YYYY-MM') as m, COALESCE(r.total, 0) as revenue
		 FROM generate_series(date_trunc('month', NOW() - interval '5 months'), date_trunc('month', NOW()), interval '1 month') as month
		 LEFT JOIN (
			SELECT date_trunc('month', created_at) as m, SUM(amount) as total
			FROM enrollments WHERE batch_id = $1 AND status = 'approved'
			GROUP BY 1
		 ) r ON r.m = month
		 ORDER BY month`,
		batchID,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var mf models.BatchMonthlyRevenue
			if err := rows.Scan(&mf.Month, &mf.Revenue); err == nil {
				stats.MonthlyData = append(stats.MonthlyData, mf)
			}
		}
	}
	if stats.MonthlyData == nil {
		stats.MonthlyData = []models.BatchMonthlyRevenue{}
	}

	erows, err := database.DB.Query(ctx,
		`SELECT e.id, COALESCE(e.course_id, 0), COALESCE(c.title,''), e.full_name, e.mobile, e.payment_method,
			e.mobile_banking, e.amount, e.sent_from, e.sent_to, e.referral_source,
			e.status, e.created_at, e.updated_at
		 FROM enrollments e
		 LEFT JOIN courses c ON e.course_id = c.id
		 WHERE e.batch_id = $1
		 ORDER BY e.created_at DESC LIMIT 5`,
		batchID,
	)
	if err == nil {
		defer erows.Close()
		for erows.Next() {
			var en models.Enrollment
			if err := erows.Scan(&en.ID, &en.CourseID, &en.CourseName, &en.FullName, &en.Mobile,
				&en.PaymentMethod, &en.MobileBanking, &en.Amount, &en.SentFrom, &en.SentTo,
				&en.ReferralSource, &en.Status, &en.CreatedAt, &en.UpdatedAt); err == nil {
				stats.RecentPayments = append(stats.RecentPayments, en)
			}
		}
	}
	if stats.RecentPayments == nil {
		stats.RecentPayments = []models.Enrollment{}
	}

	c.JSON(http.StatusOK, stats)
}

func AdminGetExpenses(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	per_page, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	category := c.Query("category")
	search := c.Query("search")

	if page < 1 {
		page = 1
	}
	if per_page < 1 {
		per_page = 20
	}
	offset := (page - 1) * per_page

	ctx := context.Background()
	where := "1=1"
	args := []interface{}{}
	argIdx := 1

	if category != "" {
		where += " AND category = $" + strconv.Itoa(argIdx)
		args = append(args, category)
		argIdx++
	}
	if search != "" {
		where += " AND (description ILIKE $" + strconv.Itoa(argIdx) + " OR notes ILIKE $" + strconv.Itoa(argIdx) + ")"
		args = append(args, "%"+search+"%")
		argIdx++
	}

	var total int
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	_ = database.DB.QueryRow(ctx, `SELECT COUNT(*) FROM expenses WHERE `+where, countArgs...).Scan(&total)

	args = append(args, per_page, offset)
	rows, err := database.DB.Query(ctx,
		`SELECT id, category, description, amount, COALESCE(date::text, ''), COALESCE(notes, ''), COALESCE(created_by, 0), created_at
		 FROM expenses WHERE `+where+` ORDER BY date DESC, id DESC LIMIT $`+strconv.Itoa(argIdx)+` OFFSET $`+strconv.Itoa(argIdx+1),
		args...,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	var expenses []models.Expense
	for rows.Next() {
		var e models.Expense
		if err := rows.Scan(&e.ID, &e.Category, &e.Description, &e.Amount, &e.Date, &e.Notes, &e.CreatedBy, &e.CreatedAt); err != nil {
			continue
		}
		expenses = append(expenses, e)
	}
	if expenses == nil {
		expenses = []models.Expense{}
	}

	c.JSON(http.StatusOK, gin.H{
		"expenses":    expenses,
		"total":       total,
		"page":        page,
		"per_page":    per_page,
		"total_pages": (total + per_page - 1) / per_page,
	})
}

func AdminCreateExpense(c *gin.Context) {
	var req models.CreateExpenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	date := req.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	var e models.Expense
	err := database.DB.QueryRow(context.Background(),
		`INSERT INTO expenses (category, description, amount, date, notes)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, category, description, amount, COALESCE(date::text, ''), COALESCE(notes, ''), COALESCE(created_by, 0), created_at`,
		req.Category, req.Description, req.Amount, date, req.Notes,
	).Scan(&e.ID, &e.Category, &e.Description, &e.Amount, &e.Date, &e.Notes, &e.CreatedBy, &e.CreatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create expense"})
		return
	}
	c.JSON(http.StatusCreated, e)
}

func AdminUpdateExpense(c *gin.Context) {
	id := c.Param("id")

	var req models.CreateExpenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	date := req.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	var e models.Expense
	err := database.DB.QueryRow(context.Background(),
		`UPDATE expenses SET category=$1, description=$2, amount=$3, date=$4, notes=$5
		 WHERE id=$6
		 RETURNING id, category, description, amount, COALESCE(date::text, ''), COALESCE(notes, ''), COALESCE(created_by, 0), created_at`,
		req.Category, req.Description, req.Amount, date, req.Notes, id,
	).Scan(&e.ID, &e.Category, &e.Description, &e.Amount, &e.Date, &e.Notes, &e.CreatedBy, &e.CreatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update expense"})
		return
	}
	c.JSON(http.StatusOK, e)
}

func AdminDeleteExpense(c *gin.Context) {
	id := c.Param("id")
	tag, err := database.DB.Exec(context.Background(), `DELETE FROM expenses WHERE id = $1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "expense not found"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "expense deleted"})
}
