package handlers

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"edunova-server/database"
	"edunova-server/models"
	"edunova-server/services"
)

const otpExpiry = 2 * time.Minute

func generateOTP() string {
	code := rand.Intn(900000) + 100000
	return fmt.Sprintf("%d", code)
}

func SendOTP(c *gin.Context) {
	var req models.SendOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	code := generateOTP()
	expiresAt := time.Now().Add(otpExpiry)

	_, err := database.DB.Exec(
		context.Background(),
		`INSERT INTO otps (mobile, code, expires_at) VALUES ($1, $2, $3)`,
		req.Mobile, code, expiresAt,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to generate OTP"})
		return
	}

	message := fmt.Sprintf("Your EduNova verification code is: %s. It will expire in 2 minutes.", code)
	if err := services.SendSMS(req.Mobile, message); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to send SMS"})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Message: fmt.Sprintf("OTP sent to %s", req.Mobile),
	})
}

func VerifyOTP(c *gin.Context) {
	var req models.VerifyOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var otp models.OTP
	err := database.DB.QueryRow(
		context.Background(),
		`SELECT id, mobile, code, expires_at, created_at
		 FROM otps
		 WHERE mobile = $1 AND code = $2
		 ORDER BY created_at DESC
		 LIMIT 1`,
		req.Mobile, req.Code,
	).Scan(&otp.ID, &otp.Mobile, &otp.Code, &otp.ExpiresAt, &otp.CreatedAt)

	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid OTP"})
		return
	}

	if time.Now().After(otp.ExpiresAt) {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "OTP expired"})
		return
	}

	result, err := database.DB.Exec(
		context.Background(),
		`UPDATE users SET verified = TRUE, updated_at = NOW() WHERE mobile = $1`,
		req.Mobile,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to verify user"})
		return
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "user not found"})
		return
	}

	_, _ = database.DB.Exec(
		context.Background(),
		`DELETE FROM otps WHERE mobile = $1`,
		req.Mobile,
	)

	c.JSON(http.StatusOK, models.SuccessResponse{
		Message: "Account verified successfully",
	})
}

func ResendOTP(c *gin.Context) {
	var req models.SendOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	_, _ = database.DB.Exec(
		context.Background(),
		`DELETE FROM otps WHERE mobile = $1`,
		req.Mobile,
	)

	code := generateOTP()
	expiresAt := time.Now().Add(otpExpiry)

	_, err := database.DB.Exec(
		context.Background(),
		`INSERT INTO otps (mobile, code, expires_at) VALUES ($1, $2, $3)`,
		req.Mobile, code, expiresAt,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to generate OTP"})
		return
	}

	message := fmt.Sprintf("Your EduNova verification code is: %s. It will expire in 2 minutes.", code)
	if err := services.SendSMS(req.Mobile, message); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to send SMS"})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Message: fmt.Sprintf("OTP resent to %s", req.Mobile),
	})
}
