package zipimages

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"virtual-exam-api/internal/media/storage"
)

const MaxZipSize = storage.MaxZipSize

// ExtractImages reads a ZIP archive and returns a map of base filename -> file bytes.
func ExtractImages(data []byte) (map[string][]byte, error) {
	if len(data) == 0 {
		return map[string][]byte{}, nil
	}
	if len(data) > MaxZipSize {
		return nil, fmt.Errorf("ไฟล์ ZIP มีขนาดใหญ่เกินไป (สูงสุด 50MB)")
	}

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("ไม่สามารถอ่านไฟล์ ZIP ได้")
	}

	out := make(map[string][]byte)
	for _, f := range reader.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if !storage.IsSafeZipEntry(f.Name) {
			continue
		}
		base := filepath.Base(strings.ReplaceAll(f.Name, "\\", "/"))
		ext := strings.ToLower(filepath.Ext(base))
		if !storage.AllowedImageExt(ext) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(io.LimitReader(rc, storage.MaxImageSize+1))
		rc.Close()
		if err != nil {
			return nil, err
		}
		if len(content) > storage.MaxImageSize {
			return nil, fmt.Errorf("ไฟล์รูปภาพ %s มีขนาดใหญ่เกินไป (สูงสุด 5MB)", base)
		}
		out[strings.ToLower(base)] = content
	}
	return out, nil
}

func LookupImage(images map[string][]byte, filename string) ([]byte, bool) {
	name := strings.ToLower(strings.TrimSpace(filepath.Base(strings.ReplaceAll(filename, "\\", "/"))))
	if name == "" {
		return nil, false
	}
	data, ok := images[name]
	return data, ok
}
