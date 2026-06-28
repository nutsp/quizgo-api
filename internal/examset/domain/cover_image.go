package domain

import (
	"net/url"
	"strings"

	"virtual-exam-api/internal/apperrors"
)

const maxCoverImageURLLength = 2048

// NormalizeCoverImageURL trims input, converts empty values to nil, and validates http/https URLs.
func NormalizeCoverImageURL(raw *string) (*string, error) {
	if raw == nil {
		return nil, nil
	}

	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		return nil, nil
	}

	if len(trimmed) > maxCoverImageURLLength {
		return nil, apperrors.ValidationError("URL รูปภาพปกไม่ถูกต้อง")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, apperrors.ValidationError("URL รูปภาพปกไม่ถูกต้อง")
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, apperrors.ValidationError("URL รูปภาพปกไม่ถูกต้อง")
	}

	return &trimmed, nil
}
