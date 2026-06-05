package handlers

import (
	"fmt"
	"net/http"
	"path"
	"time"

	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	maxImageBytes    = 5 * 1024 * 1024 // 5 MB
	presignTTL       = 5 * time.Minute
	presignExpiresIn = 300 // seconds, for response
)

var allowedContentTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

type UploadHandler struct {
	r2    *storage.R2Client // nil when R2 is not configured
	repos *repository.Repos
}

func NewUploadHandler(r2 *storage.R2Client, repos *repository.Repos) *UploadHandler {
	return &UploadHandler{r2: r2, repos: repos}
}

type presignMenuItemImageRequest struct {
	ItemID      int64  `json:"item_id" binding:"required"`
	ContentType string `json:"content_type" binding:"required"`
	FileSize    int64  `json:"file_size" binding:"required"`
}

// POST /upload/menu-item-image — staff-protected.
func (h *UploadHandler) PresignMenuItemImage(c *gin.Context) {
	if h.r2 == nil {
		respondError(c, http.StatusServiceUnavailable, CodeFeatureDisabled, "image upload is not configured")
		return
	}

	var req presignMenuItemImageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}

	ext, ok := allowedContentTypes[req.ContentType]
	if !ok {
		respondValidationError(c, "content_type must be image/jpeg, image/png, or image/webp")
		return
	}
	if req.FileSize <= 0 || req.FileSize > maxImageBytes {
		respondValidationError(c, "file_size must be between 1 and 5242880 bytes")
		return
	}

	sess, ok2 := middleware.GetStaffSession(c)
	if !ok2 {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}

	item, err := h.repos.GetMenuItemByID(c.Request.Context(), req.ItemID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeMenuItemNotFound, "item not found")
		return
	}
	if item.BranchID != sess.BranchID {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}

	key := path.Join("menu", itoa(item.BranchID), itoa(item.ID), uuid.NewString()+ext)

	uploadURL, err := h.r2.PresignPut(c.Request.Context(), key, req.ContentType, req.FileSize, presignTTL)
	if err != nil {
		respondInternalError(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"upload_url": uploadURL,
		"public_url": h.r2.PublicURL(key),
		"expires_in": presignExpiresIn,
	})
}

type presignRestaurantLogoRequest struct {
	ContentType string `json:"content_type" binding:"required"`
	FileSize    int64  `json:"file_size" binding:"required"`
}

// POST /upload/restaurant-logo — staff-protected.
func (h *UploadHandler) PresignRestaurantLogo(c *gin.Context) {
	if h.r2 == nil {
		respondError(c, http.StatusServiceUnavailable, CodeFeatureDisabled, "image upload is not configured")
		return
	}

	var req presignRestaurantLogoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}

	ext, ok := allowedContentTypes[req.ContentType]
	if !ok {
		respondValidationError(c, "content_type must be image/jpeg, image/png, or image/webp")
		return
	}
	if req.FileSize <= 0 || req.FileSize > maxImageBytes {
		respondValidationError(c, "file_size must be between 1 and 5242880 bytes")
		return
	}

	sess, ok2 := middleware.GetStaffSession(c)
	if !ok2 {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}

	restaurant, err := h.repos.GetRestaurantByBranchID(c.Request.Context(), sess.BranchID)
	if err != nil {
		respondInternalError(c)
		return
	}

	key := path.Join("restaurants", itoa(restaurant.ID), "logo"+ext)

	uploadURL, err := h.r2.PresignPut(c.Request.Context(), key, req.ContentType, req.FileSize, presignTTL)
	if err != nil {
		respondInternalError(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"upload_url": uploadURL,
		"public_url": h.r2.PublicURL(key),
		"expires_in": presignExpiresIn,
	})
}

// itoa converts an int64 to its decimal string representation.
func itoa(n int64) string {
	return fmt.Sprintf("%d", n)
}
