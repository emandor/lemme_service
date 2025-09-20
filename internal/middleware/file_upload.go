package middleware

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/emandor/lemme_service/internal/config"
	"github.com/gofiber/fiber/v2"
)

// file upload validator middleware for checking file type and size
func FileUploadValidator(cfg *config.Config) fiber.Handler {
	allowedExt := cfg.AllowedFileExt
	maxSizeMB := cfg.AllowedMaxFileSize
	extMap := make(map[string]struct{})
	for _, e := range allowedExt {
		extMap[strings.ToLower(e)] = struct{}{}
	}

	maxSize := int64(maxSizeMB) * 1024 * 1024

	return func(c *fiber.Ctx) error {
		form, err := c.MultipartForm()
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "invalid multipart form",
			})
		}

		for _, files := range form.File {
			for _, file := range files {
				if err := validateFile(file, extMap, maxSize); err != nil {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
						"error": err.Error(),
					})
				}
			}
		}

		return c.Next()
	}
}

// validateFile checks the file size and extension
func validateFile(file *multipart.FileHeader, extMap map[string]struct{}, maxSize int64) *fiber.Error {
	if file.Size > maxSize {
		return fiber.NewError(fiber.StatusRequestEntityTooLarge, "file too large")
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if _, ok := extMap[ext]; !ok {
		return fiber.NewError(fiber.StatusBadRequest, "invalid file type")
	}

	f, err := file.Open()
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "cannot open file")
	}
	defer f.Close()

	head := make([]byte, 512) // http.DetectContentType butuh <=512 byte
	n, _ := f.Read(head)
	head = head[:n]

	mimeType := http.DetectContentType(head)

	if !isValidMagic(ext, mimeType, head) {
		return fiber.NewError(fiber.StatusBadRequest, "invalid file content")
	}

	return nil
}

// verify magic numbers for jpg/jpeg and png
func isValidMagic(ext, mimeType string, head []byte) bool {
	switch ext {
	case ".jpg", ".jpeg":
		return strings.HasPrefix(mimeType, "image/jpeg") &&
			len(head) > 2 && head[0] == 0xFF && head[1] == 0xD8
	case ".png":
		return strings.HasPrefix(mimeType, "image/png") &&
			bytes.HasPrefix(head, []byte{0x89, 0x50, 0x4E, 0x47})
	default:
		return false
	}
}
