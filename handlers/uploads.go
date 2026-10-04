package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"

	"edunova-server/models"
	"edunova-server/services"
)

const maxUploadBytes = 1 << 20

var imageExtensions = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
}

var (
	userUploadPurposes  = map[string]bool{"doubt": true, "profile": true}
	adminUploadPurposes = map[string]bool{"article": true, "teacher": true}
)

func UserPresignUpload(c *gin.Context) { presignUpload(c, userUploadPurposes) }

func AdminPresignUpload(c *gin.Context) { presignUpload(c, adminUploadPurposes) }

func presignUpload(c *gin.Context, purposes map[string]bool) {
	var req models.PresignUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	ext, ok := imageExtensions[req.ContentType]
	if !ok || !purposes[req.Purpose] {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "unsupported upload type or purpose"})
		return
	}
	if req.Size <= 0 || req.Size > maxUploadBytes {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "file must be between 1 byte and 1 MB"})
		return
	}
	if !services.StorageConfigured() {
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{Error: "storage is not configured"})
		return
	}

	id := make([]byte, 16)
	_, _ = rand.Read(id)
	key := req.Purpose + "/" + hex.EncodeToString(id) + "." + ext

	uploadURL, err := services.PresignUpload(c.Request.Context(), key, req.ContentType, req.Size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to prepare upload"})
		return
	}
	c.JSON(http.StatusOK, models.PresignUploadResponse{
		UploadURL: uploadURL,
		PublicURL: services.PublicURL(key),
	})
}
