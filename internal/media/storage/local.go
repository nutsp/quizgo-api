package storage

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

const (
	MaxImageSize = 5 * 1024 * 1024
	MaxZipSize   = 50 * 1024 * 1024
)

var allowedImageMIME = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/webp": true,
}

var allowedImageExt = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".webp": true,
}

func AllowedImageExt(ext string) bool {
	return allowedImageExt[strings.ToLower(ext)]
}

type LocalStorage struct {
	rootDir string
	urlPath string
}

func NewLocalStorage(rootDir, urlPath string) (*LocalStorage, error) {
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		return nil, fmt.Errorf("create upload dir: %w", err)
	}
	if urlPath == "" {
		urlPath = "/uploads"
	}
	return &LocalStorage{rootDir: rootDir, urlPath: strings.TrimRight(urlPath, "/")}, nil
}

func (s *LocalStorage) SaveImage(subdir string, originalName string, data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("ไฟล์ว่างเปล่า")
	}
	if len(data) > MaxImageSize {
		return "", fmt.Errorf("ไฟล์รูปภาพมีขนาดใหญ่เกินไป (สูงสุด 5MB)")
	}

	base := sanitizeBaseName(originalName)
	ext := strings.ToLower(filepath.Ext(base))
	if !allowedImageExt[ext] {
		return "", fmt.Errorf("ประเภทไฟล์รูปภาพไม่รองรับ")
	}

	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = http.DetectContentType(data)
	}
	if !allowedImageMIME[mimeType] {
		return "", fmt.Errorf("ประเภทไฟล์รูปภาพไม่รองรับ")
	}

	dir := filepath.Join(s.rootDir, subdir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	storedName := uuid.New().String() + ext
	fullPath := filepath.Join(dir, storedName)
	if err := os.WriteFile(fullPath, data, 0o644); err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/%s/%s", s.urlPath, filepath.ToSlash(subdir), storedName), nil
}

func (s *LocalStorage) SaveImageReader(subdir, originalName string, r io.Reader, maxBytes int64) (string, error) {
	if maxBytes <= 0 {
		maxBytes = MaxImageSize
	}
	data, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return "", err
	}
	if int64(len(data)) > maxBytes {
		return "", fmt.Errorf("ไฟล์รูปภาพมีขนาดใหญ่เกินไป (สูงสุด 5MB)")
	}
	return s.SaveImage(subdir, originalName, data)
}

func sanitizeBaseName(name string) string {
	base := filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	base = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == 0 {
			return -1
		}
		return r
	}, base)
	if base == "" || base == "." || base == ".." {
		return "image.png"
	}
	return base
}

func IsSafeZipEntry(name string) bool {
	clean := filepath.Clean(strings.ReplaceAll(name, "\\", "/"))
	if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return false
	}
	return true
}
