package repository

import (
	"strings"
	"testing"
)

func TestLegacyReviewBackfillTargetsOnlyPublishedUnreviewedQuestions(t *testing.T) {
	for _, fragment := range []string{
		"status = 'published'",
		"review_status = 'unreviewed'",
		"review_status = 'reviewed'",
	} {
		if !strings.Contains(legacyReviewBackfillSQL, fragment) {
			t.Fatalf("legacy review backfill SQL missing %q", fragment)
		}
	}
}
