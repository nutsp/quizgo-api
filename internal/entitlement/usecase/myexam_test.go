package usecase

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"virtual-exam-api/internal/common/pagination"
	"virtual-exam-api/internal/entitlement/domain"
	examsetdomain "virtual-exam-api/internal/examset/domain"
)

func TestSortMergedItemsUsesLatestActivityFirst(t *testing.T) {
	now := time.Now().UTC()
	older := now.Add(-48 * time.Hour)
	newer := now.Add(-1 * time.Hour)

	items := []mergedItem{
		{
			set: examsetdomain.ExamSet{
				ID:        uuid.New(),
				Title:     "B",
				CreatedAt: older,
				UpdatedAt: older,
			},
		},
		{
			set: examsetdomain.ExamSet{
				ID:        uuid.New(),
				Title:     "A",
				CreatedAt: newer,
				UpdatedAt: newer,
			},
		},
	}

	sortMergedItems(items)

	if items[0].set.Title != "A" {
		t.Fatalf("expected most recently updated item first, got %q", items[0].set.Title)
	}
}

func TestListMyExamsPaginatesFilteredItems(t *testing.T) {
	allItems := []domain.MyExamItem{
		{ID: "1", AccessSource: domain.MyExamSourceSinglePurchase},
		{ID: "2", AccessSource: domain.MyExamSourcePrivateGrant},
		{
			ID:           "3",
			AccessSource: domain.MyExamSourceFreeActivity,
			LatestAttempt: &domain.MyExamLatestAttempt{
				Status: "in_progress",
			},
		},
	}

	filtered := domain.FilterMyExamItemsByTab(allItems, domain.MyExamTabInProgress)
	page := domain.SanitizeMyExamsPage(1)
	limit := domain.SanitizeMyExamsLimit(12)
	total := int64(len(filtered))
	meta := pagination.NewPaginationMeta(page, limit, total)

	if meta.Total != 1 {
		t.Fatalf("total = %d, want 1", meta.Total)
	}
	if meta.HasNext {
		t.Fatal("expected has_next=false for single item page")
	}
}
