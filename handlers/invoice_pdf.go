package handlers

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"edunova-server/models"
	"edunova-server/services"
)

// serveInvoicePDF draws the stored snapshot on demand. The PDF itself is not
// persisted: the snapshot is the record, the PDF a view of it.
func serveInvoicePDF(c *gin.Context, rec *invoiceRecord) {
	copies := 1
	if c.Query("copies") == "double" {
		copies = 2
	}

	out, err := services.RenderInvoicePDF(rec.InvoiceSnapshot, copies)
	if err != nil {
		log.Printf("invoice: pdf render for invoice %d: %v", rec.ID, err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to generate pdf"})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="%s.pdf"`, rec.Number))
	c.Header("Cache-Control", "private, max-age=300")
	c.Data(http.StatusOK, "application/pdf", out)
}
