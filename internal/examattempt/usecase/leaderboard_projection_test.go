package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	appcache "virtual-exam-api/internal/cache"
	"virtual-exam-api/internal/examattempt/domain"
	attemptrepo "virtual-exam-api/internal/examattempt/repository"
	examsetdomain "virtual-exam-api/internal/examset/domain"
	examsetrepo "virtual-exam-api/internal/examset/repository"
	leaderboarddomain "virtual-exam-api/internal/leaderboard/domain"
	qdomain "virtual-exam-api/internal/question/domain"
	questionrepo "virtual-exam-api/internal/question/repository"
	scoringuc "virtual-exam-api/internal/scoring/usecase"
)

type projectionRecorder struct {
	inputs       []leaderboarddomain.ProjectionInput
	update       *leaderboarddomain.ProjectionUpdate
	err          error
	failures     []uuid.UUID
	failureErrs  []error
	recordingErr error
}

func (r *projectionRecorder) ProjectAttempt(_ context.Context, input leaderboarddomain.ProjectionInput) (*leaderboarddomain.ProjectionUpdate, error) {
	r.inputs = append(r.inputs, input)
	return r.update, r.err
}

func (r *projectionRecorder) RecordProjectionFailure(_ context.Context, attemptID uuid.UUID, projectionErr error) error {
	r.failures = append(r.failures, attemptID)
	r.failureErrs = append(r.failureErrs, projectionErr)
	return r.recordingErr
}

type projectionAttemptStore struct {
	attemptrepo.Repository
	current         *domain.ExamAttempt
	answers         []domain.ExamAnswer
	submittedWrites int
	timeoutWrites   int
	createWrites    int
}

func (s *projectionAttemptStore) FindByIDForUser(_ context.Context, attemptID, userID uuid.UUID) (*domain.ExamAttempt, error) {
	if s.current == nil || s.current.ID != attemptID || s.current.UserID != userID {
		return nil, nil
	}
	copy := *s.current
	return &copy, nil
}

func (s *projectionAttemptStore) FindActiveAttemptByUserAndExamSet(_ context.Context, userID, examSetID uuid.UUID) (*domain.ExamAttempt, error) {
	if s.current == nil || s.current.UserID != userID || s.current.ExamSetID != examSetID || s.current.Status != domain.StatusInProgress {
		return nil, nil
	}
	copy := *s.current
	return &copy, nil
}

func (s *projectionAttemptStore) ListAnswersByAttemptID(context.Context, uuid.UUID) ([]domain.ExamAnswer, error) {
	return append([]domain.ExamAnswer(nil), s.answers...), nil
}

func (s *projectionAttemptStore) UpdateAttemptSubmitted(_ context.Context, attempt *domain.ExamAttempt, answers []domain.ExamAnswer) error {
	s.submittedWrites++
	copy := *attempt
	s.current = &copy
	s.answers = append([]domain.ExamAnswer(nil), answers...)
	return nil
}

func (s *projectionAttemptStore) MarkAttemptTimeout(_ context.Context, attemptID uuid.UUID) error {
	if s.current != nil && s.current.ID == attemptID && s.current.Status == domain.StatusInProgress {
		s.timeoutWrites++
		submittedAt := time.Date(2026, 7, 14, 7, 0, 0, 0, time.UTC)
		s.current.Status = domain.StatusTimeout
		s.current.SubmittedAt = &submittedAt
		s.current.UpdatedAt = submittedAt
	}
	return nil
}

func (s *projectionAttemptStore) CreateAttemptWithAnswers(_ context.Context, attempt *domain.ExamAttempt, answers []domain.ExamAnswer) error {
	s.createWrites++
	return nil
}

type projectionAttemptCache struct {
	attemptrepo.AttemptCacheRepository
}

func (projectionAttemptCache) ClearAttempt(context.Context, string) error { return nil }
func (projectionAttemptCache) SetAttemptState(context.Context, string, time.Duration) error {
	return nil
}
func (projectionAttemptCache) SetTimer(context.Context, string, time.Time, time.Duration) error {
	return nil
}

type projectionExamSets struct {
	examsetrepo.Repository
	set *examsetdomain.ExamSet
}

func (r *projectionExamSets) FindByID(context.Context, uuid.UUID) (*examsetdomain.ExamSet, error) {
	copy := *r.set
	return &copy, nil
}

func (r *projectionExamSets) FindByCode(_ context.Context, code string) (*examsetdomain.ExamSet, error) {
	if r.set.Code != code {
		return nil, nil
	}
	copy := *r.set
	return &copy, nil
}

type projectionQuestions struct {
	questionrepo.Repository
	items   []qdomain.ExamSetQuestion
	correct map[uuid.UUID]string
}

func (r *projectionQuestions) ListByExamSetID(context.Context, uuid.UUID) ([]qdomain.ExamSetQuestion, error) {
	return r.items, nil
}

func (r *projectionQuestions) GetCorrectChoicesByQuestionIDs(context.Context, []uuid.UUID) (map[uuid.UUID]string, error) {
	return r.correct, nil
}

func newProjectionAttemptUseCase(store *projectionAttemptStore, projector LeaderboardProjector) *ExamAttemptUseCase {
	set := &examsetdomain.ExamSet{
		ID:              store.current.ExamSetID,
		ExamTrackID:     store.current.ExamTrackID,
		Code:            "set-001",
		Title:           "Set 001",
		DurationMinutes: 60,
		TotalQuestions:  1,
		PassingScore:    60,
		Status:          examsetdomain.StatusPublished,
		IsActive:        true,
	}
	questionID := uuid.New()
	questions := &projectionQuestions{
		items: []qdomain.ExamSetQuestion{{
			ExamSetID: set.ID, QuestionID: questionID, QuestionNo: 1,
			Question: &qdomain.Question{ID: questionID, Status: qdomain.StatusPublished, IsActive: true},
		}},
		correct: map[uuid.UUID]string{questionID: qdomain.ChoiceA},
	}
	if len(store.answers) == 0 {
		selected := qdomain.ChoiceA
		store.answers = []domain.ExamAnswer{{
			ID: uuid.New(), AttemptID: store.current.ID, QuestionID: questionID, QuestionNo: 1, SelectedChoiceKey: &selected,
		}}
	} else {
		store.answers[0].QuestionID = questionID
	}
	return NewExamAttemptUseCase(
		store,
		projectionAttemptCache{},
		&projectionExamSets{set: set},
		questions,
		scoringuc.NewScoringUseCase(),
		nil,
		appcache.Noop(),
		nil,
		nil,
		nil,
		projector,
	)
}

func projectionAttempt(status string) *domain.ExamAttempt {
	startedAt := time.Now().UTC().Add(-10 * time.Minute)
	return &domain.ExamAttempt{
		ID:          uuid.New(),
		UserID:      uuid.New(),
		ExamTrackID: uuid.New(),
		ExamSetID:   uuid.New(),
		Status:      status,
		StartedAt:   startedAt,
		ExpiresAt:   startedAt.Add(time.Hour),
		ExamTrack:   &domain.ExamTrackRef{Code: "police", Name: "Police"},
	}
}

func TestLeaderboardSubmissionProjectsAfterPersistenceAndReturnsUpdate(t *testing.T) {
	attempt := projectionAttempt(domain.StatusInProgress)
	store := &projectionAttemptStore{current: attempt}
	wantUpdate := &leaderboarddomain.ProjectionUpdate{SeasonID: uuid.NewString(), CurrentRank: 12, TotalPoints: 80}
	projector := &projectionRecorder{update: wantUpdate}
	uc := newProjectionAttemptUseCase(store, projector)

	response, err := uc.Submit(context.Background(), attempt.UserID, attempt.ID)
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if response.CompetitionUpdate != wantUpdate {
		t.Fatalf("CompetitionUpdate = %#v, want %#v", response.CompetitionUpdate, wantUpdate)
	}
	if store.submittedWrites != 1 || len(projector.inputs) != 1 {
		t.Fatalf("submitted writes/project calls = %d/%d, want 1/1", store.submittedWrites, len(projector.inputs))
	}
	input := projector.inputs[0]
	if store.current.Status != domain.StatusSubmitted || input.AttemptID != attempt.ID || input.ExamTrackID != attempt.ExamTrackID {
		t.Fatalf("projection ran with invalid persisted attempt: state=%s input=%#v", store.current.Status, input)
	}
	if input.TrackCode != "police" || input.Candidate.Points != 100 || input.Candidate.DurationSeconds <= 0 {
		t.Fatalf("projection input = %#v", input)
	}
}

func TestLeaderboardProjectionFailureDoesNotFailSubmission(t *testing.T) {
	attempt := projectionAttempt(domain.StatusInProgress)
	store := &projectionAttemptStore{current: attempt}
	projectionErr := errors.New("leaderboard unavailable")
	projector := &projectionRecorder{err: projectionErr, recordingErr: errors.New("failure table unavailable")}
	uc := newProjectionAttemptUseCase(store, projector)

	response, err := uc.Submit(context.Background(), attempt.UserID, attempt.ID)
	if err != nil {
		t.Fatalf("Submit() error = %v, want successful submission", err)
	}
	if response.Status != domain.StatusSubmitted || response.CompetitionUpdate != nil {
		t.Fatalf("response = %#v", response)
	}
	if len(projector.failures) != 1 || projector.failures[0] != attempt.ID || !errors.Is(projector.failureErrs[0], projectionErr) {
		t.Fatalf("recorded failures = %#v / %#v", projector.failures, projector.failureErrs)
	}
}

func TestLeaderboardAlreadySubmittedRetryDoesNotProjectAgain(t *testing.T) {
	attempt := projectionAttempt(domain.StatusSubmitted)
	submittedAt := time.Now().UTC()
	duration := 30
	attempt.SubmittedAt = &submittedAt
	attempt.DurationSeconds = &duration
	store := &projectionAttemptStore{current: attempt}
	projector := &projectionRecorder{}
	uc := newProjectionAttemptUseCase(store, projector)

	if _, err := uc.Submit(context.Background(), attempt.UserID, attempt.ID); err != nil {
		t.Fatalf("Submit() retry error = %v", err)
	}
	if len(projector.inputs) != 0 || store.submittedWrites != 0 {
		t.Fatalf("retry projected/wrote = %d/%d, want 0/0", len(projector.inputs), store.submittedWrites)
	}
}

func TestLeaderboardExpiredAttemptProjectsTimeoutExactlyOnce(t *testing.T) {
	attempt := projectionAttempt(domain.StatusInProgress)
	attempt.StartedAt = time.Now().UTC().Add(-2 * time.Hour)
	attempt.ExpiresAt = time.Now().UTC().Add(-time.Hour)
	store := &projectionAttemptStore{current: attempt}
	projector := &projectionRecorder{}
	uc := newProjectionAttemptUseCase(store, projector)

	if err := uc.timeoutAttempt(context.Background(), attempt); err != nil {
		t.Fatalf("timeoutAttempt() error = %v", err)
	}
	if store.timeoutWrites != 1 || len(projector.inputs) != 1 {
		t.Fatalf("timeout writes/project calls = %d/%d, want 1/1", store.timeoutWrites, len(projector.inputs))
	}
	input := projector.inputs[0]
	if input.AttemptID != attempt.ID || input.Candidate.Points != 0 || input.Candidate.DurationSeconds <= 0 || input.SubmittedAt.IsZero() {
		t.Fatalf("timeout projection input = %#v", input)
	}
}

func TestLeaderboardNilProjectorIsBackwardCompatible(t *testing.T) {
	attempt := projectionAttempt(domain.StatusInProgress)
	store := &projectionAttemptStore{current: attempt}
	uc := newProjectionAttemptUseCase(store, nil)
	if _, err := uc.Submit(context.Background(), attempt.UserID, attempt.ID); err != nil {
		t.Fatalf("Submit() with nil projector error = %v", err)
	}
}

var _ LeaderboardProjector = (*projectionRecorder)(nil)
var _ attemptrepo.Repository = (*projectionAttemptStore)(nil)
var _ attemptrepo.AttemptCacheRepository = projectionAttemptCache{}
var _ examsetrepo.Repository = (*projectionExamSets)(nil)
var _ questionrepo.Repository = (*projectionQuestions)(nil)
