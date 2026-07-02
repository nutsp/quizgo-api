package domain

import "strings"

const (
	ContentFormatPlain        = "plain"
	ContentFormatMarkdownMath = "markdown_math"
)

func IsValidContentFormat(format string) bool {
	switch format {
	case ContentFormatPlain, ContentFormatMarkdownMath:
		return true
	default:
		return false
	}
}

func NormalizeContentFormat(format string) string {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		return ContentFormatPlain
	}
	if IsValidContentFormat(format) {
		return format
	}
	return ContentFormatPlain
}
