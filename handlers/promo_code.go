package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"edunova-server/database"
	"edunova-server/models"
)

// pgExecutor is satisfied by both *pgxpool.Pool and pgx.Tx, so promo-code
// validation can run either standalone (pool) or inside the enrollment
// transaction (tx) without duplicating the lookup logic.
type pgExecutor interface {
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
	Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
}

// applyPromoCode validates a code against the given course/batch/amount/mobile
// and returns the promo code's id and the computed discount (clamped to amount).
// It does not record a redemption — callers insert that themselves once the
// enrollment it's attached to actually succeeds.
func applyPromoCode(ctx context.Context, db pgExecutor, code, mobile string, courseID int, batchID *int, amount int) (int, int, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return 0, 0, errors.New("promo code is required")
	}

	var promo models.PromoCode
	err := db.QueryRow(ctx,
		`SELECT id, code, discount_type, discount_value, course_id, batch_id, max_redemptions, redemption_count, expires_at, is_active
		 FROM promo_codes WHERE code = $1`, code,
	).Scan(&promo.ID, &promo.Code, &promo.DiscountType, &promo.DiscountValue, &promo.CourseID, &promo.BatchID,
		&promo.MaxRedemptions, &promo.RedemptionCount, &promo.ExpiresAt, &promo.IsActive)
	if err != nil {
		return 0, 0, errors.New("invalid promo code")
	}

	if !promo.IsActive {
		return 0, 0, errors.New("this promo code is no longer active")
	}
	if promo.ExpiresAt != nil && promo.ExpiresAt.Before(time.Now()) {
		return 0, 0, errors.New("this promo code has expired")
	}
	if promo.MaxRedemptions != nil && promo.RedemptionCount >= *promo.MaxRedemptions {
		return 0, 0, errors.New("this promo code has reached its redemption limit")
	}
	if promo.CourseID != nil && *promo.CourseID != courseID {
		return 0, 0, errors.New("this promo code is not valid for this course")
	}
	if promo.BatchID != nil && (batchID == nil || *batchID != *promo.BatchID) {
		return 0, 0, errors.New("this promo code is not valid for this batch")
	}

	var alreadyUsed bool
	_ = db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM promo_code_redemptions WHERE promo_code_id = $1 AND mobile = $2)`,
		promo.ID, mobile,
	).Scan(&alreadyUsed)
	if alreadyUsed {
		return 0, 0, errors.New("you have already used this promo code")
	}

	discount := promo.DiscountValue
	if promo.DiscountType == "percentage" {
		discount = amount * promo.DiscountValue / 100
	}
	if discount > amount {
		discount = amount
	}
	if discount < 0 {
		discount = 0
	}

	return promo.ID, discount, nil
}

func PublicValidatePromoCode(c *gin.Context) {
	var req models.ValidatePromoCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	_, discount, err := applyPromoCode(context.Background(), database.DB, req.Code, req.Mobile, req.CourseID, req.BatchID, req.Amount)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":           true,
		"discount_amount": discount,
		"final_amount":    req.Amount - discount,
	})
}

// PublicGetAvailablePromoCodes lists active, unexpired, under-capacity promo
// codes that apply to the given course and (optionally) batch, so the
// enrollment screen can surface them before the user has a code to type in.
func PublicGetAvailablePromoCodes(c *gin.Context) {
	courseID, err := strconv.Atoi(c.Query("course_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "course_id is required"})
		return
	}

	var batchID *int
	if raw := c.Query("batch_id"); raw != "" {
		if id, err := strconv.Atoi(raw); err == nil {
			batchID = &id
		}
	}

	rows, err := database.DB.Query(context.Background(),
		`SELECT code, discount_type, discount_value, max_redemptions, redemption_count, expires_at
		 FROM promo_codes
		 WHERE is_active = true
		   AND (expires_at IS NULL OR expires_at > now())
		   AND (max_redemptions IS NULL OR redemption_count < max_redemptions)
		   AND (course_id IS NULL OR course_id = $1)
		   AND (batch_id IS NULL OR batch_id = $2)
		 ORDER BY created_at DESC`, courseID, batchID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	promos := []models.AvailablePromoCode{}
	for rows.Next() {
		var code, discountType string
		var discountValue, redemptionCount int
		var maxRedemptions *int
		var expiresAt *time.Time
		if err := rows.Scan(&code, &discountType, &discountValue, &maxRedemptions, &redemptionCount, &expiresAt); err != nil {
			continue
		}

		var remaining *int
		if maxRedemptions != nil {
			r := *maxRedemptions - redemptionCount
			remaining = &r
		}

		promos = append(promos, models.AvailablePromoCode{
			Code:          code,
			DiscountType:  discountType,
			DiscountValue: discountValue,
			Remaining:     remaining,
			ExpiresAt:     expiresAt,
		})
	}
	c.JSON(http.StatusOK, promos)
}

func AdminGetPromoCodes(c *gin.Context) {
	rows, err := database.DB.Query(context.Background(),
		`SELECT p.id, p.code, p.discount_type, p.discount_value, p.course_id, COALESCE(c.title, ''), p.batch_id, COALESCE(b.name, ''),
			p.max_redemptions, p.redemption_count, p.expires_at, p.is_active, p.created_at
		 FROM promo_codes p
		 LEFT JOIN courses c ON p.course_id = c.id
		 LEFT JOIN batches b ON p.batch_id = b.id
		 ORDER BY p.created_at DESC`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	codes := []models.PromoCode{}
	for rows.Next() {
		var p models.PromoCode
		if err := rows.Scan(&p.ID, &p.Code, &p.DiscountType, &p.DiscountValue, &p.CourseID, &p.CourseName, &p.BatchID, &p.BatchName,
			&p.MaxRedemptions, &p.RedemptionCount, &p.ExpiresAt, &p.IsActive, &p.CreatedAt); err == nil {
			codes = append(codes, p)
		}
	}
	c.JSON(http.StatusOK, codes)
}

func AdminCreatePromoCode(c *gin.Context) {
	var req models.CreatePromoCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	if req.DiscountType != "percentage" && req.DiscountType != "fixed" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "discount_type must be 'percentage' or 'fixed'"})
		return
	}

	adminIDRaw, _ := c.Get("admin_id")
	adminID, _ := adminIDRaw.(int)
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	var id int
	err := database.DB.QueryRow(context.Background(),
		`INSERT INTO promo_codes (code, discount_type, discount_value, course_id, batch_id, max_redemptions, expires_at, is_active, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
		strings.ToUpper(strings.TrimSpace(req.Code)), req.DiscountType, req.DiscountValue, req.CourseID, req.BatchID,
		req.MaxRedemptions, req.ExpiresAt, isActive, adminID,
	).Scan(&id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create promo code (code may already exist)"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": id, "message": "promo code created"})
}

func AdminUpdatePromoCode(c *gin.Context) {
	id := c.Param("id")
	var req models.CreatePromoCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	if req.DiscountType != "percentage" && req.DiscountType != "fixed" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "discount_type must be 'percentage' or 'fixed'"})
		return
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	_, err := database.DB.Exec(context.Background(),
		`UPDATE promo_codes SET code=$1, discount_type=$2, discount_value=$3, course_id=$4, batch_id=$5,
			max_redemptions=$6, expires_at=$7, is_active=$8
		 WHERE id=$9`,
		strings.ToUpper(strings.TrimSpace(req.Code)), req.DiscountType, req.DiscountValue, req.CourseID, req.BatchID,
		req.MaxRedemptions, req.ExpiresAt, isActive, id,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update promo code"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "promo code updated"})
}

func AdminDeletePromoCode(c *gin.Context) {
	id := c.Param("id")
	_, err := database.DB.Exec(context.Background(), `DELETE FROM promo_codes WHERE id=$1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to delete promo code"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "promo code deleted"})
}
