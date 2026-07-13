package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"virtual-exam-api/internal/examset/domain"
	examsetrepo "virtual-exam-api/internal/examset/repository"
	trackdomain "virtual-exam-api/internal/examtrack/domain"
	trackrepo "virtual-exam-api/internal/examtrack/repository"
	qdomain "virtual-exam-api/internal/question/domain"
	questionrepo "virtual-exam-api/internal/question/repository"
)

type lifecycleCall struct {
	kind      string
	trackID   uuid.UUID
	examSetID uuid.UUID
	at        time.Time
}

type lifecycleRecorder struct {
	calls        []lifecycleCall
	statesAtCall []domain.ExamSet
	store        *lifecycleSetStore
	err          error
}

func (r *lifecycleRecorder) OnExamSetPublished(_ context.Context, trackID, examSetID uuid.UUID, at time.Time) error {
	r.calls = append(r.calls, lifecycleCall{kind: "published", trackID: trackID, examSetID: examSetID, at: at})
	r.snapshotState()
	return r.err
}

func (r *lifecycleRecorder) OnExamSetStopped(_ context.Context, examSetID uuid.UUID, at time.Time) error {
	r.calls = append(r.calls, lifecycleCall{kind: "stopped", examSetID: examSetID, at: at})
	r.snapshotState()
	return r.err
}

func (r *lifecycleRecorder) snapshotState() {
	if r.store != nil && r.store.current != nil {
		r.statesAtCall = append(r.statesAtCall, *r.store.current)
	}
}

type lifecycleSetStore struct {
	examsetrepo.AdminRepository
	current          *domain.ExamSet
	publishAt        time.Time
	transitionAt     time.Time
	statusWrites     int
	activeWrites     int
	updateWrites     int
	deleteWrites     int
	softDelete       bool
	lifecycleAtWrite int
	pendingStops     []examsetrepo.LifecycleStopEvent
}

func (s *lifecycleSetStore) recordStop(wasEligible bool, stoppedAt time.Time) {
	if wasEligible {
		s.pendingStops = append(s.pendingStops, examsetrepo.LifecycleStopEvent{ExamSetID: s.current.ID, StoppedAt: stoppedAt})
	}
}

func (s *lifecycleSetStore) UpdateStatus(_ context.Context, _ uuid.UUID, status string, active bool) error {
	s.statusWrites++
	wasEligible := isLeaderboardEligible(s.current)
	s.current.Status = status
	s.current.IsActive = active
	s.current.UpdatedAt = s.transitionAt
	if !wasEligible && isLeaderboardEligible(s.current) {
		publishedAt := s.publishAt
		s.current.PublishedAt = &publishedAt
	}
	s.recordStop(wasEligible && !isLeaderboardEligible(s.current), s.transitionAt)
	return nil
}

func (s *lifecycleSetStore) UpdateIsActive(_ context.Context, _ uuid.UUID, active bool) error {
	s.activeWrites++
	wasEligible := isLeaderboardEligible(s.current)
	s.current.IsActive = active
	s.current.UpdatedAt = s.transitionAt
	if !wasEligible && isLeaderboardEligible(s.current) {
		publishedAt := s.publishAt
		s.current.PublishedAt = &publishedAt
	}
	s.recordStop(wasEligible && !isLeaderboardEligible(s.current), s.transitionAt)
	return nil
}

func (s *lifecycleSetStore) Update(_ context.Context, set *domain.ExamSet) error {
	s.updateWrites++
	wasEligible := isLeaderboardEligible(s.current)
	createdAt := s.current.CreatedAt
	status := s.current.Status
	*s.current = *set
	s.current.CreatedAt = createdAt
	s.current.Status = status
	s.current.UpdatedAt = s.transitionAt
	if !wasEligible && isLeaderboardEligible(s.current) {
		publishedAt := s.publishAt
		s.current.PublishedAt = &publishedAt
	}
	s.recordStop(wasEligible && !isLeaderboardEligible(s.current), s.transitionAt)
	return nil
}

func (s *lifecycleSetStore) Delete(_ context.Context, _ uuid.UUID) (bool, error) {
	s.deleteWrites++
	wasEligible := isLeaderboardEligible(s.current)
	if !s.softDelete {
		if wasEligible {
			s.pendingStops = append(s.pendingStops, examsetrepo.LifecycleStopEvent{ExamSetID: s.current.ID, StoppedAt: s.transitionAt})
		}
		s.current = nil
		return false, nil
	}
	s.current.IsActive = false
	s.current.UpdatedAt = s.transitionAt
	s.recordStop(wasEligible, s.transitionAt)
	return true, nil
}

func (s *lifecycleSetStore) ListPendingLifecycleStops(_ context.Context, examSetID uuid.UUID) ([]examsetrepo.LifecycleStopEvent, error) {
	events := make([]examsetrepo.LifecycleStopEvent, 0, len(s.pendingStops))
	for _, event := range s.pendingStops {
		if event.ExamSetID == examSetID {
			events = append(events, event)
		}
	}
	return events, nil
}

func (s *lifecycleSetStore) MarkLifecycleStopDelivered(_ context.Context, examSetID uuid.UUID, stoppedAt time.Time) error {
	remaining := s.pendingStops[:0]
	for _, event := range s.pendingStops {
		if event.ExamSetID != examSetID || !event.StoppedAt.Equal(stoppedAt) {
			remaining = append(remaining, event)
		}
	}
	s.pendingStops = remaining
	return nil
}

func (s *lifecycleSetStore) ClaimLifecycleStops(_ context.Context, request examsetrepo.LifecycleClaimRequest) ([]examsetrepo.LifecycleStopEvent, error) {
	events := make([]examsetrepo.LifecycleStopEvent, 0, len(s.pendingStops))
	for i := range s.pendingStops {
		if request.ExamSetID == nil || s.pendingStops[i].ExamSetID == *request.ExamSetID {
			s.pendingStops[i].ClaimToken = request.Token
			events = append(events, s.pendingStops[i])
		}
	}
	return events, nil
}

func (s *lifecycleSetStore) MarkLifecycleStopClaimDelivered(_ context.Context, examSetID uuid.UUID, stoppedAt time.Time, _ uuid.UUID, _ time.Time) (bool, error) {
	if err := s.MarkLifecycleStopDelivered(context.Background(), examSetID, stoppedAt); err != nil {
		return false, err
	}
	return true, nil
}

func (s *lifecycleSetStore) RetryLifecycleStop(_ context.Context, _ uuid.UUID, _ time.Time, _ uuid.UUID, _ time.Time, _ error) (bool, error) {
	return true, nil
}

type lifecycleSetReader struct {
	examsetrepo.Repository
	store *lifecycleSetStore
}

func (r *lifecycleSetReader) FindByID(_ context.Context, _ uuid.UUID) (*domain.ExamSet, error) {
	if r.store.current == nil {
		return nil, nil
	}
	copy := *r.store.current
	return &copy, nil
}

func (r *lifecycleSetReader) FindByCode(_ context.Context, code string) (*domain.ExamSet, error) {
	if r.store.current != nil && r.store.current.Code == code {
		copy := *r.store.current
		return &copy, nil
	}
	return nil, nil
}

type lifecycleTrackReader struct {
	trackrepo.Repository
	track trackdomain.ExamTrack
}

func (r *lifecycleTrackReader) FindByID(_ context.Context, _ uuid.UUID) (*trackdomain.ExamTrack, error) {
	track := r.track
	return &track, nil
}

type lifecycleTrackAdmin struct{ trackrepo.AdminRepository }

func (lifecycleTrackAdmin) RefreshCounters(context.Context, uuid.UUID) error { return nil }

type lifecycleQuestions struct {
	questionrepo.ExamSetQuestionAdminRepository
	items []qdomain.ExamSetQuestion
}

func (q *lifecycleQuestions) ListByExamSetID(context.Context, uuid.UUID) ([]qdomain.ExamSetQuestion, error) {
	return q.items, nil
}

func newLifecycleAdmin(store *lifecycleSetStore, lifecycle LeaderboardLifecycle) *AdminUseCase {
	return NewAdminUseCase(
		store,
		&lifecycleSetReader{store: store},
		&lifecycleTrackReader{track: trackdomain.ExamTrack{ID: store.current.ExamTrackID}},
		lifecycleTrackAdmin{},
		&lifecycleQuestions{items: readyQuestions(store.current.ID)},
		nil,
		lifecycle,
	)
}

func leaderboardSet() *domain.ExamSet {
	return &domain.ExamSet{
		ID:              uuid.New(),
		ExamTrackID:     uuid.New(),
		Code:            "leaderboard-set",
		Title:           "Leaderboard set",
		Description:     "Ready for publication",
		DurationMinutes: 60,
		TotalQuestions:  1,
		PassingScore:    60,
		Difficulty:      domain.DifficultyMedium,
		AccessType:      domain.AccessFree,
		Currency:        "THB",
		Mode:            domain.ModeMockExam,
		IsActive:        true,
		Status:          domain.StatusDraft,
		CreatedAt:       time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:       time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
	}
}

func lifecycleFixtureTime(hour int) time.Time {
	return time.Date(2026, 7, 14, hour, 0, 0, 0, time.UTC)
}

func readyQuestions(examSetID uuid.UUID) []qdomain.ExamSetQuestion {
	questionID := uuid.New()
	choices := []qdomain.Choice{
		{QuestionID: questionID, ChoiceKey: qdomain.ChoiceA, IsCorrect: true},
		{QuestionID: questionID, ChoiceKey: qdomain.ChoiceB},
		{QuestionID: questionID, ChoiceKey: qdomain.ChoiceC},
		{QuestionID: questionID, ChoiceKey: qdomain.ChoiceD},
	}
	return []qdomain.ExamSetQuestion{{
		ExamSetID:  examSetID,
		QuestionID: questionID,
		QuestionNo: 1,
		Question: &qdomain.Question{
			ID:          questionID,
			Status:      qdomain.StatusPublished,
			IsActive:    true,
			Explanation: "Because",
			Choices:     choices,
		},
	}}
}

func updateInputFromSet(set *domain.ExamSet) UpdateSetInput {
	return UpdateSetInput{
		ExamTrackID:         set.ExamTrackID.String(),
		Title:               set.Title,
		Code:                set.Code,
		Description:         set.Description,
		DurationMinutes:     set.DurationMinutes,
		TotalQuestions:      set.TotalQuestions,
		PassingScore:        set.PassingScore,
		Difficulty:          set.Difficulty,
		AccessType:          set.AccessType,
		AllowSinglePurchase: set.AllowSinglePurchase,
		PriceAmount:         set.PriceAmount,
		OriginalPriceAmount: set.OriginalPriceAmount,
		SalePriceAmount:     set.SalePriceAmount,
		Currency:            set.Currency,
		Mode:                set.Mode,
		IsOfficial:          set.IsOfficial,
		IsFeatured:          set.IsFeatured,
		IsActive:            set.IsActive,
	}
}

func TestLeaderboardPublishUsesPersistedPublishedAtAfterStatusWrite(t *testing.T) {
	set := leaderboardSet()
	publishedAt := lifecycleFixtureTime(2)
	store := &lifecycleSetStore{current: set, publishAt: publishedAt, transitionAt: publishedAt}
	recorder := &lifecycleRecorder{store: store}
	uc := newLifecycleAdmin(store, recorder)

	if _, err := uc.Publish(context.Background(), set.ID); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if len(recorder.calls) != 1 || recorder.calls[0].kind != "published" {
		t.Fatalf("lifecycle calls = %#v, want one publish", recorder.calls)
	}
	if recorder.calls[0].at != publishedAt {
		t.Errorf("publish timestamp = %v, want persisted %v", recorder.calls[0].at, publishedAt)
	}
	if len(recorder.statesAtCall) != 1 || recorder.statesAtCall[0].Status != domain.StatusPublished || !recorder.statesAtCall[0].IsActive {
		t.Fatalf("state at lifecycle call = %#v, want persisted published/active", recorder.statesAtCall)
	}
}

func TestLeaderboardPublishRetryReusesPublishedAt(t *testing.T) {
	set := leaderboardSet()
	publishedAt := lifecycleFixtureTime(3)
	store := &lifecycleSetStore{current: set, publishAt: publishedAt, transitionAt: publishedAt}
	recorder := &lifecycleRecorder{err: errors.New("join unavailable")}
	uc := newLifecycleAdmin(store, recorder)

	if _, err := uc.Publish(context.Background(), set.ID); err == nil {
		t.Fatal("first Publish() error = nil, want lifecycle error")
	}
	recorder.err = nil
	store.transitionAt = publishedAt.Add(time.Hour)
	if _, err := uc.Publish(context.Background(), set.ID); err != nil {
		t.Fatalf("retry Publish() error = %v", err)
	}
	if len(recorder.calls) != 2 || recorder.calls[0].at != publishedAt || recorder.calls[1].at != publishedAt {
		t.Fatalf("publish retry timestamps = %#v, want the same persisted timestamp", recorder.calls)
	}
}

func TestLeaderboardStopPathsUsePersistedTransitionTime(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(*AdminUseCase, uuid.UUID) error
	}{
		{name: "unpublish", run: func(uc *AdminUseCase, id uuid.UUID) error {
			_, err := uc.Unpublish(context.Background(), id)
			return err
		}},
		{name: "archive", run: func(uc *AdminUseCase, id uuid.UUID) error { _, err := uc.Archive(context.Background(), id); return err }},
		{name: "deactivate", run: func(uc *AdminUseCase, id uuid.UUID) error {
			_, err := uc.UpdateActiveStatus(context.Background(), id, false)
			return err
		}},
		{name: "general update", run: func(uc *AdminUseCase, id uuid.UUID) error {
			input := updateInputFromSet(uc.reads.(*lifecycleSetReader).store.current)
			input.IsActive = false
			_, err := uc.Update(context.Background(), id, input)
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			set := leaderboardSet()
			set.Status = domain.StatusPublished
			publishedAt := lifecycleFixtureTime(1).Add(-96 * time.Hour)
			set.PublishedAt = &publishedAt
			stoppedAt := lifecycleFixtureTime(4)
			store := &lifecycleSetStore{current: set, transitionAt: stoppedAt}
			recorder := &lifecycleRecorder{}
			uc := newLifecycleAdmin(store, recorder)

			if err := tc.run(uc, set.ID); err != nil {
				t.Fatalf("operation error = %v", err)
			}
			if len(recorder.calls) != 1 || recorder.calls[0].kind != "stopped" || recorder.calls[0].at != stoppedAt {
				t.Fatalf("lifecycle calls = %#v, want one stop at %v", recorder.calls, stoppedAt)
			}
		})
	}
}

func TestLeaderboardReactivationUsesAuthoritativePublishedAt(t *testing.T) {
	set := leaderboardSet()
	set.Status = domain.StatusPublished
	set.IsActive = false
	publishedAt := lifecycleFixtureTime(5)
	store := &lifecycleSetStore{current: set, publishAt: publishedAt, transitionAt: publishedAt}
	recorder := &lifecycleRecorder{}
	uc := newLifecycleAdmin(store, recorder)

	if _, err := uc.UpdateActiveStatus(context.Background(), set.ID, true); err != nil {
		t.Fatalf("UpdateActiveStatus() error = %v", err)
	}
	if len(recorder.calls) != 1 || recorder.calls[0].kind != "published" || recorder.calls[0].at != publishedAt {
		t.Fatalf("lifecycle calls = %#v, want authoritative reactivation", recorder.calls)
	}
}

func TestLeaderboardGeneralUpdateRetryKeepsStopBoundary(t *testing.T) {
	set := leaderboardSet()
	set.Status = domain.StatusPublished
	publishedAt := lifecycleFixtureTime(1).Add(-96 * time.Hour)
	set.PublishedAt = &publishedAt
	stoppedAt := lifecycleFixtureTime(6)
	store := &lifecycleSetStore{current: set, transitionAt: stoppedAt}
	recorder := &lifecycleRecorder{err: errors.New("stop unavailable")}
	uc := newLifecycleAdmin(store, recorder)
	input := updateInputFromSet(set)
	input.IsActive = false

	if _, err := uc.Update(context.Background(), set.ID, input); err == nil {
		t.Fatal("first Update() error = nil, want lifecycle error")
	}
	recorder.err = nil
	store.transitionAt = stoppedAt.Add(time.Hour)
	if _, err := uc.Update(context.Background(), set.ID, input); err != nil {
		t.Fatalf("retry Update() error = %v", err)
	}
	if store.updateWrites != 2 {
		t.Fatalf("Update repository writes = %d, want 2 retry attempts", store.updateWrites)
	}
	if len(recorder.calls) != 2 || recorder.calls[0].at != stoppedAt || recorder.calls[1].at != stoppedAt {
		t.Fatalf("update retry timestamps = %#v, want stable boundary", recorder.calls)
	}
}

func TestLeaderboardStopRetryAfterUnrelatedEditKeepsDurableBoundary(t *testing.T) {
	set := leaderboardSet()
	set.Status = domain.StatusPublished
	publishedAt := lifecycleFixtureTime(1)
	set.PublishedAt = &publishedAt
	stoppedAt := lifecycleFixtureTime(7)
	store := &lifecycleSetStore{current: set, transitionAt: stoppedAt}
	recorder := &lifecycleRecorder{err: errors.New("stop unavailable")}
	uc := newLifecycleAdmin(store, recorder)
	input := updateInputFromSet(set)
	input.IsActive = false

	if _, err := uc.Update(context.Background(), set.ID, input); err == nil {
		t.Fatal("first Update() error = nil, want lifecycle error")
	}

	recorder.err = nil
	store.transitionAt = lifecycleFixtureTime(8)
	unrelatedEdit := updateInputFromSet(store.current)
	unrelatedEdit.Title = "Edited after failed lifecycle delivery"
	if _, err := uc.Update(context.Background(), set.ID, unrelatedEdit); err != nil {
		t.Fatalf("unrelated Update() retry error = %v", err)
	}
	if len(recorder.calls) != 2 {
		t.Fatalf("lifecycle calls = %#v, want failed call plus durable retry", recorder.calls)
	}
	if recorder.calls[1].at != stoppedAt {
		t.Fatalf("retried stop timestamp = %v, want durable %v", recorder.calls[1].at, stoppedAt)
	}
}

func TestLeaderboardSoftDeleteStopsAndRetryKeepsBoundary(t *testing.T) {
	set := leaderboardSet()
	set.Status = domain.StatusPublished
	stoppedAt := lifecycleFixtureTime(9)
	store := &lifecycleSetStore{current: set, transitionAt: stoppedAt, softDelete: true}
	recorder := &lifecycleRecorder{err: errors.New("stop unavailable")}
	uc := newLifecycleAdmin(store, recorder)

	if _, err := uc.Delete(context.Background(), set.ID); err == nil {
		t.Fatal("first Delete() error = nil, want lifecycle error")
	}
	recorder.err = nil
	store.transitionAt = stoppedAt.Add(time.Hour)
	if _, err := uc.Delete(context.Background(), set.ID); err != nil {
		t.Fatalf("retry Delete() error = %v", err)
	}
	if store.deleteWrites != 2 {
		t.Fatalf("Delete repository writes = %d, want 2 retry attempts", store.deleteWrites)
	}
	if len(recorder.calls) != 2 || recorder.calls[0].at != stoppedAt || recorder.calls[1].at != stoppedAt {
		t.Fatalf("delete retry timestamps = %#v, want stable boundary", recorder.calls)
	}
}

func TestLeaderboardDeleteAfterDeactivateStillPerformsHardDelete(t *testing.T) {
	set := leaderboardSet()
	set.Status = domain.StatusPublished
	set.IsActive = false
	set.UpdatedAt = lifecycleFixtureTime(9)
	store := &lifecycleSetStore{current: set, transitionAt: lifecycleFixtureTime(10), softDelete: false}
	recorder := &lifecycleRecorder{}
	uc := newLifecycleAdmin(store, recorder)

	deactivated, err := uc.Delete(context.Background(), set.ID)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if deactivated {
		t.Fatal("Delete() deactivated = true, want hard delete")
	}
	if store.deleteWrites != 1 || store.current != nil {
		t.Fatalf("hard delete writes/current = %d/%#v, want 1/nil", store.deleteWrites, store.current)
	}
}

func TestLeaderboardLifecycleNilIsBackwardCompatible(t *testing.T) {
	set := leaderboardSet()
	now := lifecycleFixtureTime(11)
	store := &lifecycleSetStore{current: set, publishAt: now, transitionAt: now}
	uc := newLifecycleAdmin(store, nil)
	if _, err := uc.Publish(context.Background(), set.ID); err != nil {
		t.Fatalf("Publish() with nil lifecycle error = %v", err)
	}
}

var _ examsetrepo.AdminRepository = (*lifecycleSetStore)(nil)
var _ examsetrepo.Repository = (*lifecycleSetReader)(nil)
var _ trackrepo.Repository = (*lifecycleTrackReader)(nil)
var _ trackrepo.AdminRepository = lifecycleTrackAdmin{}
var _ questionrepo.ExamSetQuestionAdminRepository = (*lifecycleQuestions)(nil)
