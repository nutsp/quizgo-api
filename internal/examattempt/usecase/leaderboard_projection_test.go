package usecase

import (
	"context"
	"errors"
	"reflect"
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
	inputs         []leaderboarddomain.ProjectionInput
	update         *leaderboarddomain.ProjectionUpdate
	err            error
	failures       []uuid.UUID
	failureErrs    []error
	recordingErr   error
	store          *projectionAttemptStore
	statesAtCall   []string
	projectCtxErrs []error
	failureCtxErrs []error
}

func (r *projectionRecorder) ProjectAttempt(ctx context.Context, input leaderboarddomain.ProjectionInput) (*leaderboarddomain.ProjectionUpdate, error) {
	r.inputs = append(r.inputs, input)
	r.projectCtxErrs = append(r.projectCtxErrs, ctx.Err())
	if r.store != nil && r.store.current != nil {
		r.statesAtCall = append(r.statesAtCall, r.store.current.Status)
	}
	return r.update, r.err
}

func (r *projectionRecorder) RecordProjectionFailure(ctx context.Context, attemptID uuid.UUID, projectionErr error) error {
	r.failures = append(r.failures, attemptID)
	r.failureErrs = append(r.failureErrs, projectionErr)
	r.failureCtxErrs = append(r.failureCtxErrs, ctx.Err())
	return r.recordingErr
}

type projectionAttemptStore struct {
	attemptrepo.Repository
	current                *domain.ExamAttempt
	answers                []domain.ExamAnswer
	submittedWrites        int
	timeoutWrites          int
	createWrites           int
	findLatest             *domain.ExamAttempt
	failReloadAfterTimeout bool
	cancelAfterSubmit      context.CancelFunc
}

func (s *projectionAttemptStore) FindByIDForUser(_ context.Context, attemptID, userID uuid.UUID) (*domain.ExamAttempt, error) {
	if s.current == nil || s.current.ID != attemptID || s.current.UserID != userID {
		return nil, nil
	}
	if s.failReloadAfterTimeout && s.current.Status == domain.StatusTimeout {
		return nil, errors.New("reload unavailable")
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
	if s.cancelAfterSubmit != nil {
		s.cancelAfterSubmit()
	}
	return nil
}

func (s *projectionAttemptStore) MarkAttemptTimeout(_ context.Context, attemptID uuid.UUID) (bool, error) {
	if s.current != nil && s.current.ID == attemptID && s.current.Status == domain.StatusInProgress {
		s.timeoutWrites++
		submittedAt := projectionFixtureNow()
		s.current.Status = domain.StatusTimeout
		s.current.SubmittedAt = &submittedAt
		s.current.UpdatedAt = submittedAt
		return true, nil
	}
	return false, nil
}

func (s *projectionAttemptStore) FindLatestInProgress(context.Context, uuid.UUID) (*domain.ExamAttempt, error) {
	if s.findLatest != nil {
		copy := *s.findLatest
		return &copy, nil
	}
	if s.current == nil || s.current.Status != domain.StatusInProgress {
		return nil, nil
	}
	copy := *s.current
	return &copy, nil
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
	uc := NewExamAttemptUseCase(
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
	uc.now = projectionFixtureNow
	return uc
}

func projectionFixtureNow() time.Time {
	return time.Date(2026, 7, 14, 7, 0, 0, 0, time.UTC)
}

func projectionAttempt(status string) *domain.ExamAttempt {
	startedAt := projectionFixtureNow().Add(-10 * time.Minute)
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
	projector := &projectionRecorder{update: wantUpdate, store: store}
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
	if len(projector.statesAtCall) != 1 || projector.statesAtCall[0] != domain.StatusSubmitted {
		t.Fatalf("state at projection = %v, want submitted", projector.statesAtCall)
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
	submittedAt := projectionFixtureNow()
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
	attempt.StartedAt = projectionFixtureNow().Add(-2 * time.Hour)
	attempt.ExpiresAt = projectionFixtureNow().Add(-time.Hour)
	store := &projectionAttemptStore{current: attempt}
	projector := &projectionRecorder{store: store}
	uc := newProjectionAttemptUseCase(store, projector)

	if _, err := uc.transitionExpiredAttempt(context.Background(), attempt); err != nil {
		t.Fatalf("transitionExpiredAttempt() error = %v", err)
	}
	if store.timeoutWrites != 1 || len(projector.inputs) != 1 {
		t.Fatalf("timeout writes/project calls = %d/%d, want 1/1", store.timeoutWrites, len(projector.inputs))
	}
	input := projector.inputs[0]
	if input.AttemptID != attempt.ID || input.Candidate.Points != 0 || input.Candidate.DurationSeconds <= 0 || input.SubmittedAt.IsZero() {
		t.Fatalf("timeout projection input = %#v", input)
	}
	if len(projector.statesAtCall) != 1 || projector.statesAtCall[0] != domain.StatusTimeout {
		t.Fatalf("state at timeout projection = %v, want timeout", projector.statesAtCall)
	}
}

func TestLeaderboardConcurrentTimeoutObserversNotifyOnce(t *testing.T) {
	attempt := projectionAttempt(domain.StatusInProgress)
	attempt.ExpiresAt = projectionFixtureNow().Add(-time.Minute)
	store := &projectionAttemptStore{current: attempt}
	projector := &projectionRecorder{}
	uc := newProjectionAttemptUseCase(store, projector)
	staleCopy := *attempt

	if _, err := uc.transitionExpiredAttempt(context.Background(), attempt); err != nil {
		t.Fatalf("first transition error = %v", err)
	}
	if _, err := uc.transitionExpiredAttempt(context.Background(), &staleCopy); err != nil {
		t.Fatalf("second transition error = %v", err)
	}
	if store.timeoutWrites != 1 || len(projector.inputs) != 1 {
		t.Fatalf("timeout writes/project calls = %d/%d, want 1/1", store.timeoutWrites, len(projector.inputs))
	}
}

func TestLeaderboardTimeoutReloadFailureRecordsDurableFailure(t *testing.T) {
	attempt := projectionAttempt(domain.StatusInProgress)
	attempt.ExpiresAt = projectionFixtureNow().Add(-time.Minute)
	store := &projectionAttemptStore{current: attempt, failReloadAfterTimeout: true}
	projector := &projectionRecorder{}
	uc := newProjectionAttemptUseCase(store, projector)

	if _, err := uc.transitionExpiredAttempt(context.Background(), attempt); err == nil {
		t.Fatal("transitionExpiredAttempt() error = nil, want reload error")
	}
	if len(projector.failures) != 1 || projector.failures[0] != attempt.ID {
		t.Fatalf("recorded failures = %v, want attempt %s", projector.failures, attempt.ID)
	}
}

func TestLeaderboardExpiryGateCoversEveryAttemptEntryPoint(t *testing.T) {
	tests := []struct {
		name string
		run  func(context.Context, *ExamAttemptUseCase, *domain.ExamAttempt) error
	}{
		{name: "start", run: func(ctx context.Context, uc *ExamAttemptUseCase, attempt *domain.ExamAttempt) error {
			return callStartForCurrentSignature(ctx, uc, attempt.UserID, "set-001")
		}},
		{name: "get", run: func(ctx context.Context, uc *ExamAttemptUseCase, attempt *domain.ExamAttempt) error {
			response, err := uc.Get(ctx, attempt.UserID, attempt.ID)
			if err == nil && response.Status != domain.StatusTimeout {
				return errors.New("Get returned expired attempt as in_progress")
			}
			return err
		}},
		{name: "continue", run: func(ctx context.Context, uc *ExamAttemptUseCase, attempt *domain.ExamAttempt) error {
			response, err := uc.GetContinueAttempt(ctx, attempt.UserID)
			if err == nil && response != nil {
				return errors.New("GetContinueAttempt returned expired attempt")
			}
			return err
		}},
		{name: "submit", run: func(ctx context.Context, uc *ExamAttemptUseCase, attempt *domain.ExamAttempt) error {
			response, err := uc.Submit(ctx, attempt.UserID, attempt.ID)
			if err == nil && (response.Status != domain.StatusTimeout || response.ScorePercent != 0) {
				return errors.New("Submit scored an expired attempt")
			}
			return err
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			attempt := projectionAttempt(domain.StatusInProgress)
			attempt.StartedAt = projectionFixtureNow().Add(-2 * time.Hour)
			attempt.ExpiresAt = projectionFixtureNow().Add(-time.Minute)
			store := &projectionAttemptStore{current: attempt, findLatest: attempt}
			projector := &projectionRecorder{}
			uc := newProjectionAttemptUseCase(store, projector)

			if err := tc.run(context.Background(), uc, attempt); err != nil {
				t.Fatalf("entry point error = %v", err)
			}
			if store.timeoutWrites != 1 || len(projector.inputs) != 1 {
				t.Fatalf("timeout writes/project calls = %d/%d, want 1/1", store.timeoutWrites, len(projector.inputs))
			}
			if tc.name == "submit" && store.submittedWrites != 0 {
				t.Fatalf("submitted writes = %d, want 0", store.submittedWrites)
			}
		})
	}
}

func TestLeaderboardProjectionAndFailureRecordingOutliveCanceledRequest(t *testing.T) {
	attempt := projectionAttempt(domain.StatusInProgress)
	store := &projectionAttemptStore{current: attempt}
	projector := &projectionRecorder{err: errors.New("projection unavailable")}
	uc := newProjectionAttemptUseCase(store, projector)
	ctx, cancel := context.WithCancel(context.Background())
	store.cancelAfterSubmit = cancel

	response, err := uc.Submit(ctx, attempt.UserID, attempt.ID)
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if response.Status != domain.StatusSubmitted {
		t.Fatalf("response status = %s", response.Status)
	}
	if len(projector.projectCtxErrs) != 1 || projector.projectCtxErrs[0] != nil {
		t.Fatalf("projection context errors = %v, want nil", projector.projectCtxErrs)
	}
	if len(projector.failureCtxErrs) != 1 || projector.failureCtxErrs[0] != nil {
		t.Fatalf("failure context errors = %v, want nil", projector.failureCtxErrs)
	}
}

func callStartForCurrentSignature(ctx context.Context, uc *ExamAttemptUseCase, userID uuid.UUID, code string) error {
	method := reflect.ValueOf(uc).MethodByName("Start")
	args := []reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(userID), reflect.ValueOf(code)}
	if method.Type().NumIn() == 4 {
		args = append(args, reflect.Zero(method.Type().In(3)))
	}
	results := method.Call(args)
	if errValue := results[1]; !errValue.IsNil() {
		return errValue.Interface().(error)
	}
	return nil
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
