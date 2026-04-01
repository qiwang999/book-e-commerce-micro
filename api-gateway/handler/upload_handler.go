package handler

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/qiwang/book-e-commerce-micro/common/util"
)

const maxUploadSize = 5 << 20 // 5 MB

var allowedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
	"image/gif":  true,
}

// UploadHandler handles POST /api/v1/upload — general file upload for authenticated users.
// Query params:
//
//	category: storage sub-path (e.g. "covers", "avatars"), defaults to "general"
//
// Form data:
//
//	file: the file to upload (max 5MB, images only)
func (h *Handlers) UploadHandler(c *gin.Context) {
	if h.Storage == nil {
		util.InternalError(c, "file storage is not configured")
		return
	}

	category := c.DefaultQuery("category", "general")
	category = sanitizeCategory(category)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		util.BadRequest(c, "missing or invalid file field")
		return
	}
	defer file.Close()

	if header.Size > maxUploadSize {
		util.Error(c, http.StatusRequestEntityTooLarge, 413, fmt.Sprintf("file too large: max %d MB", maxUploadSize>>20))
		return
	}

	contentType := header.Header.Get("Content-Type")
	if !allowedImageTypes[contentType] {
		util.BadRequest(c, "unsupported file type: only jpeg, png, webp, gif are allowed")
		return
	}

	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = guessExt(contentType)
	}
	objectName := fmt.Sprintf("%s/%s%s", category, uuid.New().String(), ext)

	url, err := h.Storage.Upload(c.Request.Context(), objectName, file, header.Size, contentType)
	if err != nil {
		util.InternalError(c, "failed to upload file")
		return
	}

	util.Success(c, gin.H{"url": url})
}

// UploadBookCoverHandler handles POST /api/v1/books/upload-cover — admin-only cover upload.
// Shortcut that sets category to "covers".
func (h *Handlers) UploadBookCoverHandler(c *gin.Context) {
	c.Request.URL.RawQuery = "category=covers"
	h.UploadHandler(c)
}

func sanitizeCategory(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return -1
	}, s)
	if s == "" {
		return "general"
	}
	return s
}

func guessExt(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".bin"
	}
}
