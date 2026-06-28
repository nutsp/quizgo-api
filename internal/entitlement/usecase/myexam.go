package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"
	"virtual-exam-api/internal/cache"
	attemptdomain "virtual-exam-api/internal/examattempt/domain"
	"virtual-exam-api/internal/entitlement/domain"
	examsetdomain "virtual-exam-api/internal/examset/domain"
)

type mergedItem struct {
	set           examsetdomain.ExamSet
	source        string
	entitlement   *domain.Entitlement
	latestAttempt *attemptdomain.LatestAttemptSummary
	canStart      bool
}

func (uc *UseCase) ListMyExams(ctx context.Context, userID uuid.UUID) (*domain.MyExamsResponse, error) {
	key := cache.MyExams(userID.String())
	var cached domain.MyExamsResponse
	if ok, _ := uc.userCache.GetJSON(ctx, key, &cached); ok {
		return &cached, nil
	}

	now := time.Now().UTC()

	premiumEnt, _ := uc.entitlements.FindActivePremiumEntitlement(ctx, userID, now)
	hasPremium := premiumEnt != nil
	var premiumExpiresAt *string
	if premiumEnt != nil && premiumEnt.ExpiresAt != nil {
		s := premiumEnt.ExpiresAt.UTC().Format(time.RFC3339)
		premiumExpiresAt = &s
	}

	entRows, err := uc.entitlements.ListActiveExamSetEntitlementsByUser(ctx, userID, now)
	if err != nil {
		return nil, err
	}

	attemptRows := []attemptdomain.LatestAttemptSummary{}
	if uc.attempts != nil {
		attemptRows, err = uc.attempts.FindLatestAttemptsByUserGroupedByExamSet(ctx, userID)
		if err != nil {
			return nil, err
		}
	}

	attemptBySet := make(map[uuid.UUID]attemptdomain.LatestAttemptSummary, len(attemptRows))
	for _, row := range attemptRows {
		attemptBySet[row.ExamSetID] = row
	}

	merged := make(map[uuid.UUID]mergedItem)

	for _, row := range entRows {
		source := domain.ResolveEntitlementAccessSource(row.ExamSet.AccessType, row.Entitlement.Source)
		ent := row.Entitlement
		var latest *attemptdomain.LatestAttemptSummary
		if attempt, ok := attemptBySet[row.ExamSet.ID]; ok {
			copyAttempt := attempt
			latest = &copyAttempt
		}
		canStart := uc.canStartExamSet(ctx, userID, &row.ExamSet)
		merged[row.ExamSet.ID] = mergedItem{
			set:           row.ExamSet,
			source:        source,
			entitlement:   &ent,
			latestAttempt: latest,
			canStart:      canStart,
		}
	}

	for _, attempt := range attemptRows {
		if _, exists := merged[attempt.ExamSetID]; exists {
			continue
		}
		set, err := uc.examSets.FindByID(ctx, attempt.ExamSetID)
		if err != nil || set == nil {
			continue
		}
		if set.Status != examsetdomain.StatusPublished || !set.IsActive {
			continue
		}
		if !domain.ShouldIncludeActivityRow(set.AccessType, attempt.AccessSource) {
			continue
		}
		source := domain.ResolveActivityAccessSource(set.AccessType, derefString(attempt.AccessSource))
		copyAttempt := attempt
		canStart := uc.canStartExamSet(ctx, userID, set)
		merged[set.ID] = mergedItem{
			set:           *set,
			source:        source,
			latestAttempt: &copyAttempt,
			canStart:      canStart,
		}
	}

	items := make([]domain.MyExamItem, 0, len(merged))
	summary := domain.MyExamSummary{
		HasPremium:       hasPremium,
		PremiumExpiresAt: premiumExpiresAt,
	}

	for _, item := range merged {
		myItem := buildMyExamItem(item, hasPremium, now)
		items = append(items, myItem)
		switch myItem.AccessSource {
		case domain.MyExamSourcePrivateGrant:
			summary.PrivateExamSetCount++
			summary.GrantCount++
		case domain.MyExamSourceManualGrant:
			summary.GrantCount++
		case domain.MyExamSourceSinglePurchase:
			summary.SinglePurchaseCount++
		case domain.MyExamSourcePremiumActivity:
			summary.PremiumActivityCount++
		}
	}

	summary.UnlockedExamSetCount = summary.SinglePurchaseCount + summary.PrivateExamSetCount + summary.GrantCount

	resp := &domain.MyExamsResponse{
		Summary: summary,
		Items:   items,
	}

	_ = uc.userCache.SetJSON(ctx, key, resp, cache.TTLMyExams)
	_ = uc.userCache.AddIndex(ctx, cache.IndexUserMyExams(userID.String()), key, cache.TTLMyExams+cache.TTLIndexBuffer)

	return resp, nil
}

func (uc *UseCase) canStartExamSet(ctx context.Context, userID uuid.UUID, set *examsetdomain.ExamSet) bool {
	if uc.entitlements == nil {
		return set.Status == examsetdomain.StatusPublished && set.IsActive && set.TotalQuestions > 0
	}
	userPtr := &userID
	check := uc.CheckExamSetAccess(ctx, userPtr, set)
	return check.CanStart
}

func buildMyExamItem(item mergedItem, hasPremium bool, now time.Time) domain.MyExamItem {
	set := item.set
	myItem := domain.MyExamItem{
		ID:                  set.ID.String(),
		Code:                set.Code,
		Title:               set.Title,
		Description:         set.Description,
		AccessType:          set.AccessType,
		AccessSource:        item.source,
		SourceLabel:         domain.MyExamSourceLabel(item.source, hasPremium),
		CanStart:            item.canStart,
		CoverImageURL:       set.CoverImageURL,
		TotalQuestions:      set.TotalQuestions,
		DurationMinutes:     set.DurationMinutes,
		Difficulty:          set.Difficulty,
		PassingScore:        set.PassingScore,
		AllowSinglePurchase: set.AllowSinglePurchase,
	}
	if set.ExamTrack != nil {
		myItem.ExamTrack = &domain.MyExamTrackRef{
			ID:   set.ExamTrackID.String(),
			Name: set.ExamTrack.Name,
			Code: set.ExamTrack.Code,
		}
	}
	if item.entitlement != nil {
		ent := item.entitlement
		var expiresAt *string
		if ent.ExpiresAt != nil {
			s := ent.ExpiresAt.UTC().Format(time.RFC3339)
			expiresAt = &s
		}
		myItem.Entitlement = &domain.MyExamEntitlement{
			ID:        ent.ID.String(),
			Source:    ent.Source,
			StartsAt:  ent.StartsAt.UTC().Format(time.RFC3339),
			ExpiresAt: expiresAt,
			Status:    ent.Status(now),
		}
	}
	if item.latestAttempt != nil {
		attempt := item.latestAttempt
		latest := domain.MyExamLatestAttempt{
			AttemptID: attempt.AttemptID.String(),
			Status:    attempt.Status,
		}
		if attempt.ScorePercent != nil {
			latest.ScorePercent = attempt.ScorePercent
		}
		if attempt.SubmittedAt != nil {
			s := attempt.SubmittedAt.UTC().Format(time.RFC3339)
			latest.SubmittedAt = &s
		}
		myItem.LatestAttempt = &latest
	}
	return myItem
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
