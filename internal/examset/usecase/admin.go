package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	admindomain "virtual-exam-api/internal/admin/domain"
	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/cache"
	"virtual-exam-api/internal/common/pagination"
	"virtual-exam-api/internal/examset/domain"
	examsetrepo "virtual-exam-api/internal/examset/repository"
	trackrepo "virtual-exam-api/internal/examtrack/repository"
	questionrepo "virtual-exam-api/internal/question/repository"
)

const leaderboardLifecycleDeliveryTimeout = 5 * time.Second

type LeaderboardLifecycle interface {
	OnExamSetPublished(context.Context, uuid.UUID, uuid.UUID, time.Time) error
	OnExamSetStopped(context.Context, uuid.UUID, time.Time) error
}

type AdminUseCase struct {
	sets         examsetrepo.AdminRepository
	reads        examsetrepo.Repository
	tracks       trackrepo.Repository
	trackAdmin   trackrepo.AdminRepository
	setQuestions questionrepo.ExamSetQuestionAdminRepository
	invalidator  *cache.Invalidator
	leaderboard  LeaderboardLifecycle
}

func NewAdminUseCase(
	sets examsetrepo.AdminRepository,
	reads examsetrepo.Repository,
	tracks trackrepo.Repository,
	trackAdmin trackrepo.AdminRepository,
	setQuestions questionrepo.ExamSetQuestionAdminRepository,
	invalidator *cache.Invalidator,
	leaderboard LeaderboardLifecycle,
) *AdminUseCase {
	return &AdminUseCase{
		sets:         sets,
		reads:        reads,
		tracks:       tracks,
		trackAdmin:   trackAdmin,
		setQuestions: setQuestions,
		invalidator:  invalidator,
		leaderboard:  leaderboard,
	}
}

type AnswerSheetLayoutInput struct {
	BlockColumns      int    `json:"block_columns"`
	QuestionsPerBlock int    `json:"questions_per_block"`
	ChoiceLabelStyle  string `json:"choice_label_style"`
	ShowHeader        bool   `json:"show_header"`
	ShowInstructions  bool   `json:"show_instructions"`
	ShowCandidateInfo bool   `json:"show_candidate_info"`
}

type CreateSetInput struct {
	ExamTrackID         string                 `json:"exam_track_id"`
	Title               string                 `json:"title"`
	Code                string                 `json:"code"`
	Description         string                 `json:"description"`
	CoverImageURL       *string                `json:"cover_image_url"`
	DurationMinutes     int                    `json:"duration_minutes"`
	TotalQuestions      int                    `json:"total_questions"`
	PassingScore        int                    `json:"passing_score"`
	Difficulty          string                 `json:"difficulty"`
	AccessType          string                 `json:"access_type"`
	AllowSinglePurchase bool                   `json:"allow_single_purchase"`
	PriceAmount         float64                `json:"price_amount"`
	OriginalPriceAmount *float64               `json:"original_price_amount"`
	SalePriceAmount     *float64               `json:"sale_price_amount"`
	Currency            string                 `json:"currency"`
	Mode                string                 `json:"mode"`
	IsOfficial          bool                   `json:"is_official"`
	IsFeatured          bool                   `json:"is_featured"`
	IsActive            bool                   `json:"is_active"`
	AnswerSheetLayout   AnswerSheetLayoutInput `json:"answer_sheet_layout"`
}

type UpdateSetInput = CreateSetInput

type SetAdminResponse struct {
	domain.ExamSetSummary
	ExamTrackID string `json:"exam_track_id"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type SetListResponse = pagination.PaginatedList[SetAdminResponse]

func (uc *AdminUseCase) List(ctx context.Context, filter examsetrepo.AdminFilter) (*SetListResponse, error) {
	result, err := uc.sets.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	resp := make([]SetAdminResponse, len(result.Items))
	for i := range result.Items {
		set := result.Items[i]
		resp[i] = *toSetAdminResponse(&set)
	}
	list := pagination.NewList(resp, result.Pagination.Page, result.Pagination.Limit, result.Pagination.Total)
	return &list, nil
}

func (uc *AdminUseCase) Get(ctx context.Context, id uuid.UUID) (*SetAdminResponse, error) {
	set, err := uc.reads.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, apperrors.ErrExamSetNotFound
	}
	return toSetAdminResponse(set), nil
}

func (uc *AdminUseCase) Create(ctx context.Context, input CreateSetInput) (*SetAdminResponse, error) {
	set, err := uc.buildSetFromInput(input, nil)
	if err != nil {
		return nil, err
	}
	existing, err := uc.reads.FindByCode(ctx, set.Code)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, apperrors.ErrCodeTaken
	}
	if err := uc.sets.Create(ctx, set); err != nil {
		return nil, err
	}
	_ = uc.trackAdmin.RefreshCounters(ctx, set.ExamTrackID)
	if uc.invalidator != nil {
		uc.invalidator.OnExamSetChanged(ctx, set.ID.String(), set.Code)
	}
	return toSetAdminResponse(set), nil
}

func (uc *AdminUseCase) Update(ctx context.Context, id uuid.UUID, input UpdateSetInput) (*SetAdminResponse, error) {
	existing, err := uc.reads.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, apperrors.ErrExamSetNotFound
	}
	set, err := uc.buildSetFromInput(input, &existing.AnswerSheetLayout)
	if err != nil {
		return nil, err
	}
	set.ID = id
	set.CreatedAt = existing.CreatedAt
	set.UpdatedAt = existing.UpdatedAt
	set.PublishedAt = existing.PublishedAt
	set.Status = existing.Status
	if set.Code != existing.Code {
		byCode, err := uc.reads.FindByCode(ctx, set.Code)
		if err != nil {
			return nil, err
		}
		if byCode != nil && byCode.ID != id {
			return nil, apperrors.ErrCodeTaken
		}
	}
	if err := uc.sets.Update(ctx, set); err != nil {
		return nil, err
	}
	updated, err := uc.reads.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, apperrors.ErrExamSetNotFound
	}
	set = updated
	_ = uc.trackAdmin.RefreshCounters(ctx, set.ExamTrackID)
	if existing.ExamTrackID != set.ExamTrackID {
		_ = uc.trackAdmin.RefreshCounters(ctx, existing.ExamTrackID)
	}
	if uc.invalidator != nil {
		uc.invalidator.OnExamSetChanged(ctx, set.ID.String(), set.Code)
		if existing.Code != set.Code {
			uc.invalidator.OnExamSetChanged(ctx, set.ID.String(), existing.Code)
		}
	}
	if err := uc.syncLeaderboardLifecycle(ctx, updated); err != nil {
		return nil, err
	}
	return toSetAdminResponse(set), nil
}

func (uc *AdminUseCase) Delete(ctx context.Context, id uuid.UUID) (bool, error) {
	set, err := uc.reads.FindByID(ctx, id)
	if err != nil {
		return false, err
	}
	if set == nil {
		return false, apperrors.ErrExamSetNotFound
	}
	deactivated, err := uc.sets.Delete(ctx, id)
	if err != nil {
		return deactivated, err
	}
	if uc.invalidator != nil {
		uc.invalidator.OnExamSetChanged(ctx, set.ID.String(), set.Code)
	}
	if lifecycleErr := uc.deliverPendingLeaderboardLifecycle(ctx, id); lifecycleErr != nil {
		return deactivated, lifecycleErr
	}
	return deactivated, nil
}

func (uc *AdminUseCase) UpdateActiveStatus(ctx context.Context, id uuid.UUID, isActive bool) (*admindomain.ActiveStatusResponse, error) {
	set, err := uc.reads.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, apperrors.ErrExamSetNotFound
	}
	if err := uc.sets.UpdateIsActive(ctx, id, isActive); err != nil {
		return nil, err
	}
	updated, err := uc.reads.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, apperrors.ErrExamSetNotFound
	}
	if uc.invalidator != nil {
		uc.invalidator.OnExamSetChanged(ctx, id.String(), set.Code)
	}
	if err := uc.syncLeaderboardLifecycle(ctx, updated); err != nil {
		return nil, err
	}
	return activeStatusResponse(updated), nil
}

func isLeaderboardEligible(set *domain.ExamSet) bool {
	return set != nil && set.Status == domain.StatusPublished && set.IsActive
}

func activeStatusResponse(set *domain.ExamSet) *admindomain.ActiveStatusResponse {
	return &admindomain.ActiveStatusResponse{
		ID:        set.ID.String(),
		IsActive:  set.IsActive,
		UpdatedAt: set.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func (uc *AdminUseCase) syncLeaderboardLifecycle(ctx context.Context, set *domain.ExamSet) error {
	return uc.deliverPendingLeaderboardLifecycle(ctx, set.ID)
}

func (uc *AdminUseCase) deliverPendingLeaderboardLifecycle(ctx context.Context, examSetID uuid.UUID) error {
	if uc.leaderboard == nil {
		return nil
	}
	deliveryCtx, cancel := leaderboardLifecycleContext(ctx)
	defer cancel()
	now := time.Now().UTC()
	token := uuid.New()
	events, err := uc.sets.ClaimLifecycleEvents(deliveryCtx, examsetrepo.LifecycleClaimRequest{
		Token: token, ExamSetID: &examSetID, Limit: 20,
		Now: now, LeaseBefore: now.Add(-30 * time.Second),
	})
	if err != nil {
		return err
	}
	for _, event := range events {
		var deliveryErr error
		switch event.EventType {
		case examsetrepo.LifecycleEventPublished:
			deliveryErr = uc.leaderboard.OnExamSetPublished(deliveryCtx, event.ExamTrackID, event.ExamSetID, event.EventAt.UTC())
		case examsetrepo.LifecycleEventStopped:
			deliveryErr = uc.leaderboard.OnExamSetStopped(deliveryCtx, event.ExamSetID, event.EventAt.UTC())
		default:
			deliveryErr = errors.New("unknown exam set lifecycle event type: " + string(event.EventType))
		}
		if deliveryErr != nil {
			maintenanceCtx, maintenanceCancel := leaderboardLifecycleContext(ctx)
			_, _ = uc.sets.RetryLifecycleEvent(maintenanceCtx, event, now.Add(5*time.Second), deliveryErr)
			maintenanceCancel()
			return deliveryErr
		}
		maintenanceCtx, maintenanceCancel := leaderboardLifecycleContext(ctx)
		marked, err := uc.sets.MarkLifecycleEventDelivered(maintenanceCtx, event, time.Now().UTC())
		maintenanceCancel()
		if err != nil {
			return err
		}
		if !marked {
			return errors.New("lifecycle event claim was lost before acknowledgement")
		}
	}
	return nil
}

func leaderboardLifecycleContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), leaderboardLifecycleDeliveryTimeout)
}

func (uc *AdminUseCase) buildSetFromInput(input CreateSetInput, existingLayout *domain.AnswerSheetLayoutConfig) (*domain.ExamSet, error) {
	if input.ExamTrackID == "" || input.Title == "" || input.Code == "" {
		return nil, apperrors.ErrInvalidInput
	}
	trackID, err := uuid.Parse(input.ExamTrackID)
	if err != nil {
		return nil, apperrors.ErrInvalidUUID
	}
	track, err := uc.tracks.FindByID(context.Background(), trackID)
	if err != nil {
		return nil, err
	}
	if track == nil {
		return nil, apperrors.ErrExamTrackNotFound
	}
	if !examsetrepo.IsValidSetCode(input.Code) {
		return nil, apperrors.ErrInvalidInput
	}
	if input.DurationMinutes <= 0 || input.TotalQuestions <= 0 {
		return nil, apperrors.ErrInvalidInput
	}
	if input.PassingScore < 0 || input.PassingScore > 100 {
		return nil, apperrors.ErrInvalidInput
	}
	if !isValidDifficulty(input.Difficulty) || !isValidAccess(input.AccessType) || !isValidMode(input.Mode) {
		return nil, apperrors.ErrInvalidInput
	}
	if err := domain.ValidateAccessConfig(input.AccessType, input.PriceAmount, input.SalePriceAmount, input.AllowSinglePurchase); err != nil {
		return nil, err
	}
	input.PriceAmount, input.AllowSinglePurchase = domain.NormalizeAccessConfig(
		input.AccessType,
		input.PriceAmount,
		input.AllowSinglePurchase,
	)
	if input.AccessType == domain.AccessPrivate {
		input.PriceAmount = 0
		input.SalePriceAmount = nil
		input.OriginalPriceAmount = nil
		input.AllowSinglePurchase = false
	}
	if input.AccessType == domain.AccessFree || input.AccessType == domain.AccessTrial {
		input.PriceAmount = 0
		input.AllowSinglePurchase = false
		input.OriginalPriceAmount = nil
		input.SalePriceAmount = nil
	}
	if input.AccessType == domain.AccessPremium && !input.AllowSinglePurchase {
		input.OriginalPriceAmount = nil
	}
	currency := input.Currency
	if currency == "" {
		currency = "THB"
	}
	layout := domain.DefaultAnswerSheetLayout()
	if existingLayout != nil {
		layout = *existingLayout
	}
	coverImageURL, err := domain.NormalizeCoverImageURL(input.CoverImageURL)
	if err != nil {
		return nil, err
	}
	return &domain.ExamSet{
		ExamTrackID:         trackID,
		Code:                input.Code,
		Title:               input.Title,
		Description:         input.Description,
		CoverImageURL:       coverImageURL,
		DurationMinutes:     input.DurationMinutes,
		TotalQuestions:      input.TotalQuestions,
		PassingScore:        input.PassingScore,
		Difficulty:          input.Difficulty,
		AccessType:          input.AccessType,
		AllowSinglePurchase: input.AllowSinglePurchase,
		PriceAmount:         input.PriceAmount,
		OriginalPriceAmount: input.OriginalPriceAmount,
		Currency:            currency,
		SalePriceAmount:     input.SalePriceAmount,
		Mode:                input.Mode,
		IsOfficial:          input.IsOfficial,
		IsFeatured:          input.IsFeatured,
		IsActive:            input.IsActive,
		Status:              domain.StatusDraft,
		AnswerSheetLayout:   layout,
		ExamTrack:           &domain.ExamTrackRef{Code: track.Code, Name: track.Name},
	}, nil
}

func toSetAdminResponse(set *domain.ExamSet) *SetAdminResponse {
	summary := set.ToSummary()
	return &SetAdminResponse{
		ExamSetSummary: summary,
		ExamTrackID:    set.ExamTrackID.String(),
		CreatedAt:      set.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:      set.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func isValidDifficulty(d string) bool {
	return d == domain.DifficultyEasy || d == domain.DifficultyMedium || d == domain.DifficultyHard
}

func isValidAccess(a string) bool {
	return a == domain.AccessFree || a == domain.AccessTrial || a == domain.AccessPaid || a == domain.AccessPremium || a == domain.AccessPrivate
}

func isValidMode(m string) bool {
	return m == domain.ModePractice || m == domain.ModeMockExam
}
