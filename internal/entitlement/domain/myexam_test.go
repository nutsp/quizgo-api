package domain

import "testing"

func TestMatchesMyExamTab(t *testing.T) {
	inProgress := MyExamItem{
		AccessSource: MyExamSourceFreeActivity,
		LatestAttempt: &MyExamLatestAttempt{
			Status: "in_progress",
		},
	}
	completed := MyExamItem{
		AccessSource: MyExamSourcePremiumActivity,
		LatestAttempt: &MyExamLatestAttempt{
			Status: "submitted",
		},
	}
	unlocked := MyExamItem{
		AccessSource: MyExamSourceSinglePurchase,
	}
	special := MyExamItem{
		AccessSource: MyExamSourcePrivateGrant,
	}
	manual := MyExamItem{
		AccessSource: MyExamSourceManualGrant,
	}

	tests := []struct {
		tab  string
		item MyExamItem
		want bool
	}{
		{MyExamTabAll, inProgress, true},
		{MyExamTabInProgress, inProgress, true},
		{MyExamTabInProgress, completed, false},
		{MyExamTabCompleted, completed, true},
		{MyExamTabCompleted, inProgress, false},
		{MyExamTabUnlocked, unlocked, true},
		{MyExamTabUnlocked, manual, true},
		{MyExamTabUnlocked, special, false},
		{MyExamTabSpecialGrant, special, true},
		{MyExamTabSpecialGrant, manual, true},
		{MyExamTabSpecialGrant, unlocked, false},
	}

	for _, tt := range tests {
		if got := MatchesMyExamTab(tt.item, tt.tab); got != tt.want {
			t.Fatalf("MatchesMyExamTab(%q) = %v, want %v", tt.tab, got, tt.want)
		}
	}
}

func TestFilterMyExamItemsByTab(t *testing.T) {
	items := []MyExamItem{
		{ID: "1", AccessSource: MyExamSourceSinglePurchase},
		{ID: "2", AccessSource: MyExamSourcePrivateGrant},
		{
			ID:           "3",
			AccessSource: MyExamSourceFreeActivity,
			LatestAttempt: &MyExamLatestAttempt{
				Status: "in_progress",
			},
		},
	}

	filtered := FilterMyExamItemsByTab(items, MyExamTabInProgress)
	if len(filtered) != 1 || filtered[0].ID != "3" {
		t.Fatalf("unexpected filtered items: %+v", filtered)
	}
}

func TestSanitizeMyExamsLimit(t *testing.T) {
	if got := SanitizeMyExamsLimit(0); got != MyExamsDefaultLimit {
		t.Fatalf("default limit = %d, want %d", got, MyExamsDefaultLimit)
	}
	if got := SanitizeMyExamsLimit(100); got != MyExamsMaxLimit {
		t.Fatalf("max limit = %d, want %d", got, MyExamsMaxLimit)
	}
}

func TestNormalizeMyExamTab(t *testing.T) {
	if got := NormalizeMyExamTab("unknown"); got != MyExamTabAll {
		t.Fatalf("normalize unknown tab = %q, want %q", got, MyExamTabAll)
	}
}
