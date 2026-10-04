package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"edunova-server/models"
	"edunova-server/services"
)

type textField struct{ label, value string }

// requireEnglishText rejects Bangla letters in fields that are printed on
// invoices: the PDF renderer cannot shape Bangla, so it would print broken
// names. It writes the 400 response itself and reports whether to continue.
func requireEnglishText(c *gin.Context, fields ...textField) bool {
	for _, f := range fields {
		if services.ContainsBangla(f.value) {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error: fmt.Sprintf("%s must be written in English letters. ইংরেজি অক্ষরে লিখুন: %s", f.label, f.label),
			})
			return false
		}
	}
	return true
}
