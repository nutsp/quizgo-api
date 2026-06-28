package cache

import "context"

type Invalidator struct {
	content CacheService
	user    CacheService
	result  CacheService
}

func NewInvalidator(content, user, result CacheService) *Invalidator {
	return &Invalidator{
		content: content,
		user:    user,
		result:  result,
	}
}

func (inv *Invalidator) OnExamTrackChanged(ctx context.Context) {
	_ = inv.content.DeleteByIndex(ctx, IndexExamTracks())
	_ = inv.content.DeleteByIndex(ctx, IndexExamSetsList())
	_ = inv.content.DeleteByIndex(ctx, IndexHome())
}

func (inv *Invalidator) OnExamSetChanged(ctx context.Context, examSetID, examSetCode string) {
	if examSetID != "" {
		_ = inv.content.DeleteByIndex(ctx, IndexExamSet(examSetID))
	}
	if examSetCode != "" {
		_ = inv.content.DeleteByIndex(ctx, IndexExamSetCode(examSetCode))
	}
	_ = inv.content.DeleteByIndex(ctx, IndexExamSetsList())
	_ = inv.content.DeleteByIndex(ctx, IndexHome())
}

func (inv *Invalidator) OnUserAccessChanged(ctx context.Context, userID string) {
	if userID == "" {
		return
	}
	_ = inv.user.DeleteByIndex(ctx, IndexUserAccess(userID))
	_ = inv.user.DeleteByIndex(ctx, IndexUserMyExams(userID))
	_ = inv.user.Delete(ctx, UserEntitlements(userID), MyExams(userID))
}

func (inv *Invalidator) OnAttemptResultChanged(ctx context.Context, attemptID string) {
	if attemptID == "" {
		return
	}
	_ = inv.result.DeleteByIndex(ctx, IndexAttemptResult(attemptID))
}
