package usecase

import (
	"context"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"virtual-exam-api/internal/apperrors"
	appcache "virtual-exam-api/internal/cache"
	entdomain "virtual-exam-api/internal/entitlement/domain"
	entitlementuc "virtual-exam-api/internal/entitlement/usecase"
	"virtual-exam-api/internal/examattempt/domain"
	attemptrepo "virtual-exam-api/internal/examattempt/repository"
	examsetdomain "virtual-exam-api/internal/examset/domain"
	examsetrepo "virtual-exam-api/internal/examset/repository"
	qdomain "virtual-exam-api/internal/question/domain"
	questionrepo "virtual-exam-api/internal/question/repository"
	scoringdomain "virtual-exam-api/internal/scoring/domain"
	scoringuc "virtual-exam-api/internal/scoring/usecase"
)

type ExamAttemptUseCase struct {
	attempts     attemptrepo.Repository
	cache        attemptrepo.AttemptCacheRepository
	examSets     examsetrepo.Repository
	questions    questionrepo.Repository
	scoring      *scoringuc.ScoringUseCase
	entitlements *entitlementuc.UseCase
	resultCache  appcache.CacheService
	runtimeLocks *appcache.RuntimeLocks
	invalidator  *appcache.Invalidator
	validator    *validator.Validate
}

func NewExamAttemptUseCase(
	attempts attemptrepo.Repository,
	attemptCache attemptrepo.AttemptCacheRepository,
	examSets examsetrepo.Repository,
	questions questionrepo.Repository,
	scoring *scoringuc.ScoringUseCase,
	entitlements *entitlementuc.UseCase,
	resultCache appcache.CacheService,
	runtimeLocks *appcache.RuntimeLocks,
	invalidator *appcache.Invalidator,
) *ExamAttemptUseCase {
	if resultCache == nil {
		resultCache = appcache.Noop()
	}
	return &ExamAttemptUseCase{
		attempts:     attempts,
		cache:        attemptCache,
		examSets:     examSets,
		questions:    questions,
		scoring:      scoring,
		entitlements: entitlements,
		resultCache:  resultCache,
		runtimeLocks: runtimeLocks,
		invalidator:  invalidator,
		validator:    validator.New(),
	}
}

func (uc *ExamAttemptUseCase) Start(ctx context.Context, userID uuid.UUID, examSetCode string) (*domain.StartAttemptResponse, error) {
	set, err := uc.examSets.FindByCode(ctx, examSetCode)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, apperrors.ErrExamSetNotFound
	}

	setQuestions, err := uc.questions.ListByExamSetID(ctx, set.ID)
	if err != nil {
		return nil, err
	}

	if set.Status != examsetdomain.StatusPublished || !set.IsActive || len(setQuestions) == 0 {
		return nil, apperrors.ErrExamNotAvailable
	}

	existing, err := uc.attempts.FindActiveAttemptByUserAndExamSet(ctx, userID, set.ID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if existing != nil {
		if now.After(existing.ExpiresAt) {
			_ = uc.attempts.MarkAttemptTimeout(ctx, existing.ID)
			uc.invalidateUserExams(ctx, userID)
		} else {
			return uc.buildStartResponseFromExisting(ctx, existing, set, setQuestions)
		}
	}

	var check entdomain.ExamSetAccessResult
	if uc.entitlements != nil {
		userIDPtr := &userID
		check = uc.entitlements.CheckExamSetAccessWithQuestionCount(ctx, userIDPtr, set, len(setQuestions))
		if !check.CanStart {
			return nil, uc.entitlements.AccessDeniedError(set, check)
		}
	}

	if !uc.runtimeLocks.TryDuplicateCreateLock(ctx, userID.String(), set.ID.String()) {
		return nil, apperrors.ErrDuplicateRequest
	}
	defer uc.runtimeLocks.ReleaseDuplicateCreateLock(ctx, userID.String(), set.ID.String())

	expiresAt := now.Add(time.Duration(set.DurationMinutes) * time.Minute)
	attemptID := uuid.New()

	attempt := &domain.ExamAttempt{
		ID:          attemptID,
		UserID:      userID,
		ExamTrackID: set.ExamTrackID,
		ExamSetID:   set.ID,
		Status:      domain.StatusInProgress,
		StartedAt:   now,
		ExpiresAt:   expiresAt,
	}
	uc.applyAccessSnapshot(attempt, check, now)

	answers := make([]domain.ExamAnswer, len(setQuestions))
	for i, sq := range setQuestions {
		answers[i] = domain.ExamAnswer{
			ID:         uuid.New(),
			AttemptID:  attemptID,
			QuestionID: sq.QuestionID,
			QuestionNo: sq.QuestionNo,
		}
	}

	if err := uc.attempts.CreateAttemptWithAnswers(ctx, attempt, answers); err != nil {
		return nil, err
	}

	ttl := attemptrepo.AttemptTTL(set.DurationMinutes)
	_ = uc.cache.SetAttemptState(ctx, attemptID.String(), ttl)
	_ = uc.cache.SetTimer(ctx, attemptID.String(), expiresAt, ttl)
	uc.invalidateUserExams(ctx, userID)

	return &domain.StartAttemptResponse{
		AttemptID: attemptID.String(),
		ExamSet:   buildExamSetRef(set),
		StartedAt: now,
		ExpiresAt: expiresAt,
		Questions: buildQuestionsForExam(setQuestions),
		Answers:   map[int]string{},
	}, nil
}

func (uc *ExamAttemptUseCase) applyAccessSnapshot(attempt *domain.ExamAttempt, check entdomain.ExamSetAccessResult, now time.Time) {
	if check.AccessSource == "" {
		source := entdomain.AccessSourceFree
		attempt.AccessSource = &source
		return
	}
	source := check.AccessSource
	attempt.AccessSource = &source
	attempt.AccessEntitlementID = check.EntitlementID
	attempt.AccessGrantedAt = &now
	attempt.AccessExpiresAt = check.AccessExpiresAt
}

func (uc *ExamAttemptUseCase) buildStartResponseFromExisting(
	ctx context.Context,
	attempt *domain.ExamAttempt,
	set *examsetdomain.ExamSet,
	setQuestions []qdomain.ExamSetQuestion,
) (*domain.StartAttemptResponse, error) {
	answers, err := uc.attempts.ListAnswersByAttemptID(ctx, attempt.ID)
	if err != nil {
		return nil, err
	}
	answerMap, _ := buildAnswerMap(answers)
	return &domain.StartAttemptResponse{
		AttemptID: attempt.ID.String(),
		ExamSet:   buildExamSetRef(set),
		StartedAt: attempt.StartedAt,
		ExpiresAt: attempt.ExpiresAt,
		Questions: buildQuestionsForExam(setQuestions),
		Answers:   answerMap,
	}, nil
}

func (uc *ExamAttemptUseCase) invalidateUserExams(ctx context.Context, userID uuid.UUID) {
	if uc.invalidator != nil {
		uc.invalidator.OnUserAccessChanged(ctx, userID.String())
	}
}

func (uc *ExamAttemptUseCase) Get(ctx context.Context, userID, attemptID uuid.UUID) (*domain.GetAttemptResponse, error) {
	attempt, err := uc.getOwnedAttempt(ctx, userID, attemptID)
	if err != nil {
		return nil, err
	}

	setQuestions, err := uc.questions.ListByExamSetID(ctx, attempt.ExamSetID)
	if err != nil {
		return nil, err
	}

	answers, err := uc.attempts.ListAnswersByAttemptID(ctx, attemptID)
	if err != nil {
		return nil, err
	}

	answerMap, answeredCount := buildAnswerMap(answers)
	examSetRef := domain.ExamSetRef{}
	if attempt.ExamSet != nil {
		examSetRef = *attempt.ExamSet
	}

	return &domain.GetAttemptResponse{
		AttemptID:        attempt.ID.String(),
		Status:           attempt.Status,
		ExamSet:          examSetRef,
		StartedAt:        attempt.StartedAt,
		ExpiresAt:        attempt.ExpiresAt,
		RemainingSeconds: attemptrepo.RemainingSeconds(attempt.ExpiresAt),
		Questions:        buildQuestionsForExam(setQuestions),
		Answers:          answerMap,
		AnsweredCount:    answeredCount,
		UnansweredCount:  len(setQuestions) - answeredCount,
	}, nil
}

func (uc *ExamAttemptUseCase) SaveAnswer(ctx context.Context, userID, attemptID uuid.UUID, questionNo int, req domain.SaveAnswerRequest) (*domain.SaveAnswerResponse, error) {
	if err := uc.validator.Struct(req); err != nil {
		return nil, apperrors.ErrInvalidInput
	}
	if !qdomain.IsValidChoiceKey(req.SelectedChoiceKey) {
		return nil, apperrors.ErrInvalidChoiceKey
	}

	attempt, err := uc.getEditableAttempt(ctx, userID, attemptID)
	if err != nil {
		return nil, err
	}

	answers, err := uc.attempts.ListAnswersByAttemptID(ctx, attemptID)
	if err != nil {
		return nil, err
	}

	var target *domain.ExamAnswer
	for i := range answers {
		if answers[i].QuestionNo == questionNo {
			target = &answers[i]
			break
		}
	}
	if target == nil {
		return nil, apperrors.ErrQuestionNotFound
	}

	now := time.Now().UTC()
	choice := req.SelectedChoiceKey
	target.SelectedChoiceKey = &choice
	target.AnsweredAt = &now

	if err := uc.attempts.UpsertAnswer(ctx, target); err != nil {
		return nil, err
	}

	set, _ := uc.examSets.FindByID(ctx, attempt.ExamSetID)
	ttl := attemptrepo.AttemptTTL(120)
	if set != nil {
		ttl = attemptrepo.AttemptTTL(set.DurationMinutes)
	}
	_ = uc.cache.SaveAnswer(ctx, attemptID.String(), questionNo, req.SelectedChoiceKey, ttl)

	answeredCount := 0
	for _, a := range answers {
		if a.QuestionNo == questionNo {
			continue
		}
		if a.SelectedChoiceKey != nil && *a.SelectedChoiceKey != "" {
			answeredCount++
		}
	}
	answeredCount++

	total := len(answers)
	return &domain.SaveAnswerResponse{
		QuestionNo:        questionNo,
		SelectedChoiceKey: req.SelectedChoiceKey,
		AnsweredCount:     answeredCount,
		UnansweredCount:   total - answeredCount,
		MarkedCount:       0,
	}, nil
}

func (uc *ExamAttemptUseCase) ClearAnswer(ctx context.Context, userID, attemptID uuid.UUID, questionNo int) (*domain.SaveAnswerResponse, error) {
	if _, err := uc.getEditableAttempt(ctx, userID, attemptID); err != nil {
		return nil, err
	}

	answers, err := uc.attempts.ListAnswersByAttemptID(ctx, attemptID)
	if err != nil {
		return nil, err
	}

	found := false
	for _, a := range answers {
		if a.QuestionNo == questionNo {
			found = true
			break
		}
	}
	if !found {
		return nil, apperrors.ErrQuestionNotFound
	}

	if err := uc.attempts.ClearAnswer(ctx, attemptID, questionNo); err != nil {
		return nil, err
	}
	_ = uc.cache.RemoveAnswer(ctx, attemptID.String(), questionNo)

	answeredCount := 0
	for _, a := range answers {
		if a.QuestionNo == questionNo {
			continue
		}
		if a.SelectedChoiceKey != nil && *a.SelectedChoiceKey != "" {
			answeredCount++
		}
	}

	total := len(answers)
	return &domain.SaveAnswerResponse{
		QuestionNo:       questionNo,
		AnsweredCount:    answeredCount,
		UnansweredCount:  total - answeredCount,
	}, nil
}

func (uc *ExamAttemptUseCase) Submit(ctx context.Context, userID, attemptID uuid.UUID) (*domain.SubmitResponse, error) {
	if !uc.runtimeLocks.TrySubmitLock(ctx, attemptID.String()) {
		attempt, err := uc.attempts.FindByIDForUser(ctx, attemptID, userID)
		if err != nil {
			return nil, err
		}
		if attempt == nil {
			return nil, apperrors.ErrAttemptNotFound
		}
		if attempt.Status == domain.StatusSubmitted || attempt.Status == domain.StatusTimeout {
			return uc.buildSubmitResponse(attempt), nil
		}
		return nil, apperrors.ErrDuplicateRequest
	}
	defer uc.runtimeLocks.ReleaseSubmitLock(ctx, attemptID.String())

	attempt, err := uc.attempts.FindByIDForUser(ctx, attemptID, userID)
	if err != nil {
		return nil, err
	}
	if attempt == nil {
		return nil, apperrors.ErrAttemptNotFound
	}

	if attempt.Status == domain.StatusSubmitted || attempt.Status == domain.StatusTimeout {
		return uc.buildSubmitResponse(attempt), nil
	}
	if attempt.Status != domain.StatusInProgress {
		return nil, apperrors.ErrAttemptNotEditable
	}

	set, err := uc.examSets.FindByID(ctx, attempt.ExamSetID)
	if err != nil {
		return nil, err
	}

	setQuestions, err := uc.questions.ListByExamSetID(ctx, attempt.ExamSetID)
	if err != nil {
		return nil, err
	}

	answers, err := uc.attempts.ListAnswersByAttemptID(ctx, attemptID)
	if err != nil {
		return nil, err
	}

	questionIDs := make([]uuid.UUID, len(answers))
	for i, a := range answers {
		questionIDs[i] = a.QuestionID
	}
	correctChoices, err := uc.questions.GetCorrectChoicesByQuestionIDs(ctx, questionIDs)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	scoreInputs := make([]scoringuc.AnswerInput, len(answers))
	for i, a := range answers {
		scoreInputs[i] = scoringuc.AnswerInput{
			QuestionID:        a.QuestionID,
			QuestionNo:        a.QuestionNo,
			SelectedChoiceKey: a.SelectedChoiceKey,
		}
	}
	scoreInputs = scoringuc.BuildAnswerInputs(setQuestions, scoreInputs)

	passingScore := 60
	if set != nil {
		passingScore = set.PassingScore
	}

	result := uc.scoring.Calculate(scoringuc.ScoreInput{
		Answers:        scoreInputs,
		CorrectChoices: correctChoices,
		TotalQuestions: len(setQuestions),
		PassingScore:   passingScore,
		StartedAt:      attempt.StartedAt,
		SubmittedAt:    now,
	})

	for i := range answers {
		isCorrect := false
		if answers[i].SelectedChoiceKey != nil {
			if key, ok := correctChoices[answers[i].QuestionID]; ok && *answers[i].SelectedChoiceKey == key {
				isCorrect = true
			}
		}
		answers[i].IsCorrect = &isCorrect
	}

	submittedAt := now
	duration := result.DurationSeconds
	attempt.Status = domain.StatusSubmitted
	attempt.SubmittedAt = &submittedAt
	attempt.DurationSeconds = &duration
	attempt.Score = result.Score
	attempt.TotalScore = result.TotalScore
	attempt.ScorePercent = result.ScorePercent
	attempt.CorrectCount = result.CorrectCount
	attempt.WrongCount = result.WrongCount
	attempt.UnansweredCount = result.UnansweredCount

	if err := uc.attempts.UpdateAttemptSubmitted(ctx, attempt, answers); err != nil {
		return nil, err
	}

	_ = uc.cache.ClearAttempt(ctx, attemptID.String())
	_ = uc.resultCache.DeleteByIndex(ctx, appcache.IndexAttemptResult(attemptID.String()))
	uc.invalidateUserExams(ctx, userID)

	return uc.buildSubmitResponse(attempt), nil
}

func (uc *ExamAttemptUseCase) GetResult(ctx context.Context, userID, attemptID uuid.UUID) (*domain.ResultResponse, error) {
	attempt, err := uc.getSubmittedAttempt(ctx, userID, attemptID)
	if err != nil {
		return nil, err
	}

	key := appcache.ResultSummary(attemptID.String())
	var cached domain.ResultResponse
	if ok, _ := uc.resultCache.GetJSON(ctx, key, &cached); ok {
		return &cached, nil
	}

	result, err := uc.buildResultResponse(ctx, attempt)
	if err != nil {
		return nil, err
	}

	_ = uc.resultCache.SetJSON(ctx, key, result, appcache.TTLResult)
	_ = uc.resultCache.AddIndex(ctx, appcache.IndexAttemptResult(attemptID.String()), key, appcache.TTLResult+appcache.TTLIndexBuffer)

	return result, nil
}

func (uc *ExamAttemptUseCase) buildResultResponse(ctx context.Context, attempt *domain.ExamAttempt) (*domain.ResultResponse, error) {
	attemptID := attempt.ID

	setQuestions, err := uc.questions.ListByExamSetID(ctx, attempt.ExamSetID)
	if err != nil {
		return nil, err
	}

	answers, err := uc.attempts.ListAnswersByAttemptID(ctx, attemptID)
	if err != nil {
		return nil, err
	}

	questionIDs := make([]uuid.UUID, len(answers))
	for i, a := range answers {
		questionIDs[i] = a.QuestionID
	}
	correctChoices, err := uc.questions.GetCorrectChoicesByQuestionIDs(ctx, questionIDs)
	if err != nil {
		return nil, err
	}

	scoreInputs := make([]scoringuc.AnswerInput, len(answers))
	for i, a := range answers {
		scoreInputs[i] = scoringuc.AnswerInput{
			QuestionID:        a.QuestionID,
			QuestionNo:        a.QuestionNo,
			SelectedChoiceKey: a.SelectedChoiceKey,
		}
	}
	scoreInputs = scoringuc.BuildAnswerInputs(setQuestions, scoreInputs)
	breakdown := uc.scoring.SubjectBreakdown(scoreInputs, correctChoices)
	weakness := uc.scoring.WeaknessAnalysis(breakdown, 3)

	examSetRef := domain.ExamSetRef{}
	if attempt.ExamSet != nil {
		examSetRef = *attempt.ExamSet
	}

	examTrackRef := domain.ExamTrackRef{}
	if attempt.ExamTrack != nil {
		examTrackRef = *attempt.ExamTrack
	}

	duration := 0
	if attempt.DurationSeconds != nil {
		duration = *attempt.DurationSeconds
	}

	passed := int(attempt.ScorePercent) >= examSetRef.PassingScore

	return &domain.ResultResponse{
		AttemptID: attempt.ID.String(),
		ExamSet:   examSetRef,
		ExamTrack: examTrackRef,
		Summary: domain.ResultSummary{
			Status:          attempt.Status,
			Score:           attempt.Score,
			TotalScore:      attempt.TotalScore,
			ScorePercent:    attempt.ScorePercent,
			Passed:          passed,
			CorrectCount:    attempt.CorrectCount,
			WrongCount:      attempt.WrongCount,
			UnansweredCount: attempt.UnansweredCount,
			DurationSeconds: duration,
			StartedAt:       attempt.StartedAt,
			SubmittedAt:     attempt.SubmittedAt,
		},
		SubjectBreakdown: mapSubjectBreakdown(breakdown),
		WeaknessAnalysis: mapWeaknessAnalysis(weakness),
	}, nil
}

func (uc *ExamAttemptUseCase) GetReview(ctx context.Context, userID, attemptID uuid.UUID) (*domain.ReviewResponse, error) {
	attempt, err := uc.getSubmittedAttempt(ctx, userID, attemptID)
	if err != nil {
		return nil, err
	}

	key := appcache.ResultReview(attemptID.String())
	var cached domain.ReviewResponse
	if ok, _ := uc.resultCache.GetJSON(ctx, key, &cached); ok {
		return &cached, nil
	}

	review, err := uc.buildReviewResponse(ctx, attempt)
	if err != nil {
		return nil, err
	}

	_ = uc.resultCache.SetJSON(ctx, key, review, appcache.TTLResult)
	_ = uc.resultCache.AddIndex(ctx, appcache.IndexAttemptResult(attemptID.String()), key, appcache.TTLResult+appcache.TTLIndexBuffer)

	return review, nil
}

func (uc *ExamAttemptUseCase) buildReviewResponse(ctx context.Context, attempt *domain.ExamAttempt) (*domain.ReviewResponse, error) {
	attemptID := attempt.ID
	rows, err := uc.attempts.ListAnswersWithQuestions(ctx, attemptID)
	if err != nil {
		return nil, err
	}

	questions := make([]domain.QuestionForReview, 0, len(rows))
	for _, row := range rows {
		correctKey := ""
		selectedKey := row.Answer.SelectedChoiceKey
		isUnanswered := selectedKey == nil || *selectedKey == ""
		reviewChoices := make([]domain.ReviewChoice, 0, len(row.Question.Choices))
		for _, c := range row.Question.Choices {
			if c.IsCorrect {
				correctKey = c.ChoiceKey
			}
			isSelected := selectedKey != nil && *selectedKey == c.ChoiceKey
			reviewChoices = append(reviewChoices, domain.ReviewChoice{
				ChoiceKey:   c.ChoiceKey,
				ChoiceLabel: c.ChoiceLabel,
				ChoiceText:  c.ChoiceText,
				IsSelected:  isSelected,
				IsCorrect:   c.IsCorrect,
			})
		}
		isCorrect := false
		if row.Answer.IsCorrect != nil {
			isCorrect = *row.Answer.IsCorrect
		}
		reviewTags := make([]domain.ReviewTagRef, len(row.Question.Tags))
		for i, t := range row.Question.Tags {
			reviewTags[i] = domain.ReviewTagRef{Name: t.Name, Code: t.Code}
		}
		questions = append(questions, domain.QuestionForReview{
			QuestionNo:        row.Answer.QuestionNo,
			QuestionID:        row.Answer.QuestionID.String(),
			QuestionText:      row.Question.QuestionText,
			Choices:           reviewChoices,
			SelectedChoiceKey: selectedKey,
			CorrectChoiceKey:  correctKey,
			IsCorrect:         isCorrect,
			IsUnanswered:      isUnanswered,
			Explanation:       row.Question.Explanation,
			Subject:           row.Question.SubjectName,
			Tags:              reviewTags,
		})
	}

	examSetRef := domain.ExamSetRef{}
	if attempt.ExamSet != nil {
		examSetRef = *attempt.ExamSet
	}

	return &domain.ReviewResponse{
		AttemptID: attempt.ID.String(),
		ExamSet:   examSetRef,
		Questions: questions,
	}, nil
}

func (uc *ExamAttemptUseCase) GetContinueAttempt(ctx context.Context, userID uuid.UUID) (*domain.ContinueAttempt, error) {
	attempt, err := uc.attempts.FindLatestInProgress(ctx, userID)
	if err != nil {
		return nil, err
	}
	if attempt == nil {
		return nil, nil
	}
	if time.Now().UTC().After(attempt.ExpiresAt) {
		return nil, nil
	}

	answers, err := uc.attempts.ListAnswersByAttemptID(ctx, attempt.ID)
	if err != nil {
		return nil, err
	}
	_, answeredCount := buildAnswerMap(answers)

	title := ""
	code := ""
	total := 0
	if attempt.ExamSet != nil {
		title = attempt.ExamSet.Title
		code = attempt.ExamSet.Code
		total = attempt.ExamSet.TotalQuestions
	}

	return &domain.ContinueAttempt{
		AttemptID:        attempt.ID.String(),
		ExamSetCode:      code,
		ExamSetTitle:     title,
		AnsweredCount:    answeredCount,
		TotalQuestions:   total,
		RemainingSeconds: attemptrepo.RemainingSeconds(attempt.ExpiresAt),
		ExpiresAt:        attempt.ExpiresAt,
	}, nil
}

func (uc *ExamAttemptUseCase) getOwnedAttempt(ctx context.Context, userID, attemptID uuid.UUID) (*domain.ExamAttempt, error) {
	attempt, err := uc.attempts.FindByIDForUser(ctx, attemptID, userID)
	if err != nil {
		return nil, err
	}
	if attempt == nil {
		return nil, apperrors.ErrAttemptNotFound
	}
	return attempt, nil
}

func (uc *ExamAttemptUseCase) getEditableAttempt(ctx context.Context, userID, attemptID uuid.UUID) (*domain.ExamAttempt, error) {
	attempt, err := uc.getOwnedAttempt(ctx, userID, attemptID)
	if err != nil {
		return nil, err
	}
	if attempt.Status != domain.StatusInProgress {
		return nil, apperrors.ErrAttemptSubmitted
	}
	if time.Now().UTC().After(attempt.ExpiresAt) {
		return nil, apperrors.ErrAttemptExpired
	}
	return attempt, nil
}

func (uc *ExamAttemptUseCase) getSubmittedAttempt(ctx context.Context, userID, attemptID uuid.UUID) (*domain.ExamAttempt, error) {
	attempt, err := uc.getOwnedAttempt(ctx, userID, attemptID)
	if err != nil {
		return nil, err
	}
	if attempt.Status != domain.StatusSubmitted && attempt.Status != domain.StatusTimeout {
		return nil, apperrors.ErrResultNotAvailable
	}
	return attempt, nil
}

func (uc *ExamAttemptUseCase) buildSubmitResponse(attempt *domain.ExamAttempt) *domain.SubmitResponse {
	duration := 0
	if attempt.DurationSeconds != nil {
		duration = *attempt.DurationSeconds
	}
	passed := false
	if attempt.ExamSet != nil {
		passed = int(attempt.ScorePercent) >= attempt.ExamSet.PassingScore
	}
	return &domain.SubmitResponse{
		AttemptID:       attempt.ID.String(),
		Status:          attempt.Status,
		Score:           attempt.Score,
		TotalScore:      attempt.TotalScore,
		ScorePercent:    attempt.ScorePercent,
		CorrectCount:    attempt.CorrectCount,
		WrongCount:      attempt.WrongCount,
		UnansweredCount: attempt.UnansweredCount,
		DurationSeconds: duration,
		Passed:          passed,
	}
}

func buildQuestionsForExam(setQuestions []qdomain.ExamSetQuestion) []domain.QuestionForExam {
	out := make([]domain.QuestionForExam, 0, len(setQuestions))
	for _, sq := range setQuestions {
		if sq.Question == nil {
			continue
		}
		out = append(out, domain.QuestionForExam{
			QuestionNo:   sq.QuestionNo,
			QuestionID:   sq.QuestionID.String(),
			QuestionText: sq.Question.QuestionText,
			Choices:      mapChoices(sq.Question.Choices),
		})
	}
	return out
}

func mapChoices(choices []qdomain.Choice) []domain.ChoicePublic {
	out := make([]domain.ChoicePublic, len(choices))
	for i, c := range choices {
		out[i] = domain.ChoicePublic{
			ChoiceKey:   c.ChoiceKey,
			ChoiceLabel: c.ChoiceLabel,
			ChoiceText:  c.ChoiceText,
		}
	}
	return out
}

func buildAnswerMap(answers []domain.ExamAnswer) (map[int]string, int) {
	m := make(map[int]string)
	count := 0
	for _, a := range answers {
		if a.SelectedChoiceKey != nil && *a.SelectedChoiceKey != "" {
			m[a.QuestionNo] = *a.SelectedChoiceKey
			count++
		}
	}
	return m, count
}

func mapSubjectBreakdown(items []scoringdomain.SubjectScore) []domain.SubjectBreakdown {
	out := make([]domain.SubjectBreakdown, len(items))
	for i, s := range items {
		out[i] = domain.SubjectBreakdown{
			SubjectName:  s.SubjectName,
			Correct:      s.Correct,
			Wrong:        s.Wrong,
			Unanswered:   s.Unanswered,
			Total:        s.Total,
			ScorePercent: s.ScorePercent,
		}
	}
	return out
}

func mapWeaknessAnalysis(items []scoringdomain.SubjectScore) []domain.WeaknessAnalysisItem {
	out := make([]domain.WeaknessAnalysisItem, len(items))
	for i, s := range items {
		out[i] = domain.WeaknessAnalysisItem{
			SubjectName:    s.SubjectName,
			ScorePercent:   s.ScorePercent,
			Recommendation: "ควรฝึกข้อสอบหมวดนี้เพิ่ม",
		}
	}
	return out
}

func buildExamSetRef(set *examsetdomain.ExamSet) domain.ExamSetRef {
	layout := set.AnswerSheetLayout
	if err := layout.Validate(); err != nil {
		layout = examsetdomain.DefaultAnswerSheetLayout()
	}
	return domain.ExamSetRef{
		Code:              set.Code,
		Title:             set.Title,
		DurationMinutes:   set.DurationMinutes,
		TotalQuestions:    set.TotalQuestions,
		PassingScore:      set.PassingScore,
		AnswerSheetLayout: layout,
	}
}
