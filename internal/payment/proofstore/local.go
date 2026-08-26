package proofstore

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"virtual-exam-api/internal/payment/domain"
)

const MaxProofSize int64 = 10 * 1024 * 1024

var allowedMIME = map[string]string{
	"image/jpeg":      ".jpg",
	"image/png":       ".png",
	"image/webp":      ".webp",
	"application/pdf": ".pdf",
}

type Local struct{ root string }

func NewLocal(root string) (*Local, error) {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	return &Local{root: root}, nil
}

func (s *Local) Save(_ context.Context, paymentID uuid.UUID, originalName string, src io.Reader, declaredSize int64) (domain.Proof, error) {
	if declaredSize <= 0 || declaredSize > MaxProofSize {
		return domain.Proof{}, domain.ErrInvalidProof
	}
	data, err := io.ReadAll(io.LimitReader(src, MaxProofSize+1))
	if err != nil || len(data) == 0 || int64(len(data)) > MaxProofSize {
		return domain.Proof{}, domain.ErrInvalidProof
	}
	mimeType := http.DetectContentType(data)
	ext, ok := allowedMIME[mimeType]
	if !ok {
		return domain.Proof{}, domain.ErrInvalidProof
	}
	requestedExt := strings.ToLower(filepath.Ext(filepath.Base(strings.ReplaceAll(originalName, "\\", "/"))))
	if !extensionMatchesMIME(requestedExt, mimeType) {
		return domain.Proof{}, domain.ErrInvalidProof
	}
	dir := filepath.Join(s.root, paymentID.String())
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return domain.Proof{}, err
	}
	name := uuid.NewString() + ext
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
		return domain.Proof{}, err
	}
	return domain.Proof{
		StorageKey: paymentID.String() + "/" + name, OriginalName: filepath.Base(originalName),
		MIMEType: mimeType, Size: int64(len(data)),
	}, nil
}

func (s *Local) Open(_ context.Context, storageKey string) (io.ReadCloser, error) {
	clean := filepath.Clean(strings.ReplaceAll(storageKey, "\\", "/"))
	if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return nil, domain.ErrForbidden
	}
	full := filepath.Join(s.root, clean)
	rel, err := filepath.Rel(s.root, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		return nil, domain.ErrForbidden
	}
	file, err := os.Open(full)
	if errors.Is(err, os.ErrNotExist) {
		return nil, domain.ErrNotFound
	}
	return file, err
}

func extensionMatchesMIME(ext, mimeType string) bool {
	if mimeType == "image/jpeg" {
		return ext == ".jpg" || ext == ".jpeg"
	}
	want, ok := allowedMIME[mimeType]
	return ok && ext == want
}
