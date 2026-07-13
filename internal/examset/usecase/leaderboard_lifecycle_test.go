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
	calls []lifecycleCall
	err   error
}

func (r *lifecycleRecorder) OnExamSetPublished(_ context.Context, trackID, examSetID uuid.UUID, at time.Time) error {
	r.calls = append(r.calls, lifecycleCall{kind: "published", trackID: trackID, examSetID: examSetID, at: at})
	return r.err
}

func (r *lifecycleRecorder) OnExamSetStopped(_ context.Context, examSetID uuid.UUID, at time.Time) error {
	r.calls = append(r.calls, lifecycleCall{kind: "stopped", examSetID: examSetID, at: at})
	return r.err
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
	return nil
}

func (s *lifecycleSetStore) Delete(_ context.Context, _ uuid.UUID) (bool, error) {
	s.deleteWrites++
	if !s.softDelete {
		s.current = nil
		return false, nil
	}
	s.current.IsActive = false
	s.current.UpdatedAt = s.transitionAt
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
	publishedAt := time.Date(2026, 7, 14, 2, 0, 0, 0, time.UTC)
	store := &lifecycleSetStore{current: set, publishAt: publishedAt, transitionAt: publishedAt}
	recorder := &lifecycleRecorder{}
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
	if store.current.Status != domain.StatusPublished || !store.current.IsActive {
		t.Fatalf("lifecycle ran before persistence: state = %s/%v", store.current.Status, store.current.IsActive)
	}
}

func TestLeaderboardPublishRetryReusesPublishedAt(t *testing.T) {
	set := leaderboardSet()
	publishedAt := time.Date(2026, 7, 14, 2, 15, 0, 0, time.UTC)
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
			publishedAt := time.Date(2026, 7, 10, 1, 0, 0, 0, time.UTC)
			set.PublishedAt = &publishedAt
			stoppedAt := time.Date(2026, 7, 14, 3, 0, 0, 0, time.UTC)
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
	publishedAt := time.Date(2026, 7, 14, 4, 0, 0, 0, time.UTC)
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
	publishedAt := time.Date(2026, 7, 10, 1, 0, 0, 0, time.UTC)
	set.PublishedAt = &publishedAt
	stoppedAt := time.Date(2026, 7, 14, 4, 30, 0, 0, time.UTC)
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
	if store.updateWrites != 1 {
		t.Fatalf("Update repository writes = %d, want 1", store.updateWrites)
	}
	if len(recorder.calls) != 2 || recorder.calls[0].at != stoppedAt || recorder.calls[1].at != stoppedAt {
		t.Fatalf("update retry timestamps = %#v, want stable boundary", recorder.calls)
	}
}

func TestLeaderboardSoftDeleteStopsAndRetryKeepsBoundary(t *testing.T) {
	set := leaderboardSet()
	set.Status = domain.StatusPublished
	stoppedAt := time.Date(2026, 7, 14, 5, 0, 0, 0, time.UTC)
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
	if store.deleteWrites != 1 {
		t.Fatalf("Delete repository writes = %d, want 1", store.deleteWrites)
	}
	if len(recorder.calls) != 2 || recorder.calls[0].at != stoppedAt || recorder.calls[1].at != stoppedAt {
		t.Fatalf("delete retry timestamps = %#v, want stable boundary", recorder.calls)
	}
}

func TestLeaderboardLifecycleNilIsBackwardCompatible(t *testing.T) {
	set := leaderboardSet()
	store := &lifecycleSetStore{current: set, publishAt: time.Now().UTC(), transitionAt: time.Now().UTC()}
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
