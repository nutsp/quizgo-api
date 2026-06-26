package domain

import (
	"testing"
	"time"
)

func TestBuildAccessSummary(t *testing.T) {
	expires := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		hasPremium bool
		examSets   int
		expiresAt  *time.Time
		wantType   string
	}{
		{"free", false, 0, nil, AccessTypeFree},
		{"exam set only", false, 3, nil, AccessTypeExamSet},
		{"premium only", true, 0, &expires, AccessTypePremium},
		{"premium plus exam set shows premium only", true, 2, &expires, AccessTypePremium},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildAccessSummary(tt.hasPremium, tt.examSets, tt.expiresAt)
			if got.DisplayAccessType != tt.wantType {
				t.Fatalf("display_access_type = %q, want %q", got.DisplayAccessType, tt.wantType)
			}
			if got.HasPremium != tt.hasPremium {
				t.Fatalf("has_premium = %v, want %v", got.HasPremium, tt.hasPremium)
			}
			if got.ActiveExamSetCount != tt.examSets {
				t.Fatalf("active_exam_set_count = %d, want %d", got.ActiveExamSetCount, tt.examSets)
			}
		})
	}
}
