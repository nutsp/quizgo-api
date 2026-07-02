package mathconvert

import (
	"regexp"
	"strings"
)

var (
	reDateSlash  = regexp.MustCompile(`\d{1,2}/\d{1,2}/\d{2,4}`)
	reSqrt       = regexp.MustCompile(`sqrt\(\s*([^)]+?)\s*\)`)
	rePower      = regexp.MustCompile(`([a-zA-Z0-9]+)\^([a-zA-Z0-9]+)`)
	reSimpleFrac = regexp.MustCompile(`(\d{1,3})\s*/\s*(\d{1,3})`)
	reHasLatex   = regexp.MustCompile(`\$[^$]+\$`)
)

// ShouldConvert returns true when row is marked as math content.
func ShouldConvert(questionType, contentFormat string) bool {
	qt := strings.ToLower(strings.TrimSpace(questionType))
	cf := strings.ToLower(strings.TrimSpace(contentFormat))
	return qt == "math" || cf == "markdown_math"
}

// ConvertSimpleMath wraps simple math patterns in inline LaTeX for markdown_math rendering.
// Existing $...$ segments are preserved.
func ConvertSimpleMath(text string) string {
	text = strings.TrimSpace(text)
	if text == "" || reHasLatex.MatchString(text) {
		return text
	}

	out := text
	out = reSqrt.ReplaceAllString(out, `$\\sqrt{$1}$`)
	out = rePower.ReplaceAllStringFunc(out, func(m string) string {
		parts := rePower.FindStringSubmatch(m)
		if len(parts) != 3 {
			return m
		}
		return "$" + parts[1] + "^{" + parts[2] + "}$"
	})
	out = convertFractions(out)
	return out
}

func convertFractions(text string) string {
	if reDateSlash.MatchString(text) {
		return text
	}
	return reSimpleFrac.ReplaceAllStringFunc(text, func(m string) string {
		parts := reSimpleFrac.FindStringSubmatch(m)
		if len(parts) != 3 {
			return m
		}
		return `$\\frac{` + parts[1] + `}{` + parts[2] + `}$`
	})
}
