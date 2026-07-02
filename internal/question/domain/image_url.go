package domain

import (
	"net/url"
	"strings"
)

const maxImageURLLength = 2048

// NormalizeImageURL validates and normalizes question/choice image URLs.
// Accepts https/http absolute URLs or app-relative /uploads/ paths.
func NormalizeImageURL(raw string) (*string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if len(raw) > maxImageURLLength {
		return nil, errInvalidImageURL
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(lower, "data:") {
		return nil, errInvalidImageURL
	}
	if strings.HasPrefix(raw, "/uploads/") {
		return &raw, nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, errInvalidImageURL
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errInvalidImageURL
	}
	return &raw, nil
}

var errInvalidImageURL = &validationError{msg: "URL รูปภาพไม่ถูกต้อง"}

type validationError struct {
	msg string
}

func (e *validationError) Error() string { return e.msg }

func ImageURLValidationMessage(err error) string {
	if err == nil {
		return ""
	}
	if ve, ok := err.(*validationError); ok {
		return ve.msg
	}
	return "URL รูปภาพไม่ถูกต้อง"
}
