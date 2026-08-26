package domain

import "testing"

func TestQuestionReviewStatusValidation(t *testing.T) {
	for _, status := range []string{ReviewStatusUnreviewed, ReviewStatusReviewed, ReviewStatusNeedsReview} {
		if !IsValidReviewStatus(status) {
			t.Fatalf("expected %q to be valid", status)
		}
	}
	if IsValidReviewStatus("approved-by-ai") {
		t.Fatal("unexpected review status accepted")
	}
}
