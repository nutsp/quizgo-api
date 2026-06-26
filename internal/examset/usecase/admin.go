package usecase

import (
	"context"

	"github.com/google/uuid"
	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/examset/domain"
	examsetrepo "virtual-exam-api/internal/examset/repository"
	questionrepo "virtual-exam-api/internal/question/repository"
	trackrepo "virtual-exam-api/internal/examtrack/repository"
)

type AdminUseCase struct {
	sets          examsetrepo.AdminRepository
	reads         examsetrepo.Repository
	tracks        trackrepo.Repository
	trackAdmin    trackrepo.AdminRepository
	setQuestions  questionrepo.ExamSetQuestionAdminRepository
}

func NewAdminUseCase(
	sets examsetrepo.AdminRepository,
	reads examsetrepo.Repository,
	tracks trackrepo.Repository,
	trackAdmin trackrepo.AdminRepository,
	setQuestions questionrepo.ExamSetQuestionAdminRepository,
) *AdminUseCase {
	return &AdminUseCase{
		sets:         sets,
		reads:        reads,
		tracks:       tracks,
		trackAdmin:   trackAdmin,
		setQuestions: setQuestions,
	}
}

type CreateSetInput struct {
	ExamTrackID     string   `json:"exam_track_id"`
	Title           string   `json:"title"`
	Code            string   `json:"code"`
	Description     string   `json:"description"`
	CoverImageURL   *string  `json:"cover_image_url"`
	DurationMinutes int      `json:"duration_minutes"`
	TotalQuestions  int      `json:"total_questions"`
	PassingScore    int      `json:"passing_score"`
	Difficulty      string   `json:"difficulty"`
	AccessType      string   `json:"access_type"`
	PriceAmount     float64  `json:"price_amount"`
	SalePriceAmount *float64 `json:"sale_price_amount"`
	Currency        string   `json:"currency"`
	Mode            string   `json:"mode"`
	IsOfficial      bool     `json:"is_official"`
	IsFeatured      bool     `json:"is_featured"`
	IsActive        bool     `json:"is_active"`
}

type UpdateSetInput = CreateSetInput

type SetAdminResponse struct {
	domain.ExamSetSummary
	ExamTrackID string `json:"exam_track_id"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func (uc *AdminUseCase) List(ctx context.Context, filter examsetrepo.AdminFilter) (*domain.PaginatedResult, error) {
	return uc.sets.List(ctx, filter)
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
	set, err := uc.buildSetFromInput(input)
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
	set, err := uc.buildSetFromInput(input)
	if err != nil {
		return nil, err
	}
	set.ID = id
	set.CreatedAt = existing.CreatedAt
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
	_ = uc.trackAdmin.RefreshCounters(ctx, set.ExamTrackID)
	if existing.ExamTrackID != set.ExamTrackID {
		_ = uc.trackAdmin.RefreshCounters(ctx, existing.ExamTrackID)
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
	return uc.sets.Delete(ctx, id)
}

func (uc *AdminUseCase) buildSetFromInput(input CreateSetInput) (*domain.ExamSet, error) {
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
	if input.AccessType == domain.AccessFree && input.PriceAmount != 0 {
		return nil, apperrors.ErrInvalidInput
	}
	if input.AccessType == domain.AccessPremium && input.PriceAmount < 0 {
		return nil, apperrors.ErrInvalidInput
	}
	currency := input.Currency
	if currency == "" {
		currency = "THB"
	}
	return &domain.ExamSet{
		ExamTrackID:     trackID,
		Code:            input.Code,
		Title:           input.Title,
		Description:     input.Description,
		CoverImageURL:   input.CoverImageURL,
		DurationMinutes: input.DurationMinutes,
		TotalQuestions:  input.TotalQuestions,
		PassingScore:    input.PassingScore,
		Difficulty:      input.Difficulty,
		AccessType:      input.AccessType,
		PriceAmount:     input.PriceAmount,
		Currency:        currency,
		SalePriceAmount: input.SalePriceAmount,
		Mode:            input.Mode,
		IsOfficial:      input.IsOfficial,
		IsFeatured:      input.IsFeatured,
		IsActive:        input.IsActive,
		Status:          domain.StatusDraft,
		ExamTrack:       &domain.ExamTrackRef{Code: track.Code, Name: track.Name},
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
	return a == domain.AccessFree || a == domain.AccessPremium
}

func isValidMode(m string) bool {
	return m == domain.ModePractice || m == domain.ModeMockExam
}
