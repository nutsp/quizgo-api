package domain

import "testing"

func TestNormalizeCoverImageURL(t *testing.T) {
	valid := "https://example.com/cover.jpg"
	out, err := NormalizeCoverImageURL(&valid)
	if err != nil || out == nil || *out != valid {
		t.Fatalf("expected valid URL, got %v err %v", out, err)
	}

	empty := ""
	out, err = NormalizeCoverImageURL(&empty)
	if err != nil || out != nil {
		t.Fatalf("expected nil for empty string, got %v err %v", out, err)
	}

	js := "javascript:alert(1)"
	out, err = NormalizeCoverImageURL(&js)
	if err == nil || out != nil {
		t.Fatalf("expected error for javascript URL")
	}

	long := "https://example.com/" + stringsRepeat("a", 2048)
	out, err = NormalizeCoverImageURL(&long)
	if err == nil || out != nil {
		t.Fatalf("expected error for long URL")
	}
}

func stringsRepeat(s string, n int) string {
	b := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		b = append(b, s...)
	}
	return string(b)
}
