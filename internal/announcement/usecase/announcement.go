package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"
	"virtual-exam-api/internal/announcement/domain"
	announcementrepo "virtual-exam-api/internal/announcement/repository"
	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/cache"
	"virtual-exam-api/internal/common/pagination"
	examsetrepo "virtual-exam-api/internal/examset/repository"
	trackrepo "virtual-exam-api/internal/examtrack/repository"
)

type UseCase struct {
	repository  announcementrepo.Repository
	tracks      trackrepo.Repository
	examSets    examsetrepo.Repository
	cache       cache.CacheService
	invalidator *cache.Invalidator
	now         func() time.Time
}

func NewUseCase(repository announcementrepo.Repository, tracks trackrepo.Repository, examSets examsetrepo.Repository, contentCache cache.CacheService, invalidator *cache.Invalidator) *UseCase {
	if contentCache == nil {
		contentCache = cache.Noop()
	}
	return &UseCase{repository: repository, tracks: tracks, examSets: examSets, cache: contentCache, invalidator: invalidator, now: time.Now}
}

type MutationInput struct {
	Title           string               `json:"title"`
	Slug            string               `json:"slug"`
	Summary         string               `json:"summary"`
	Content         string               `json:"content"`
	Type            domain.Type          `json:"type"`
	Priority        int                  `json:"priority"`
	IsPinned        bool                 `json:"is_pinned"`
	IsActive        bool                 `json:"is_active"`
	PublishStatus   domain.PublishStatus `json:"publish_status"`
	StartsAt        *time.Time           `json:"starts_at"`
	EndsAt          *time.Time           `json:"ends_at"`
	ExamTrackID     *uuid.UUID           `json:"exam_track_id"`
	ExamDate        *string              `json:"exam_date"`
	DaysBeforeStart int                  `json:"days_before_start"`
	CTALabel        string               `json:"cta_label"`
	CTAURL          string               `json:"cta_url"`
	ExamSetIDs      []uuid.UUID          `json:"exam_set_ids"`
}

type StatusInput struct {
	PublishStatus domain.PublishStatus `json:"publish_status"`
}

type ExamTrackResponse struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

type ExamSetResponse struct {
	ID    string `json:"id"`
	Code  string `json:"code"`
	Title string `json:"title"`
}

type AnnouncementResponse struct {
	ID                  string               `json:"id"`
	Title               string               `json:"title"`
	Slug                string               `json:"slug"`
	Summary             string               `json:"summary,omitempty"`
	Content             string               `json:"content,omitempty"`
	Type                domain.Type          `json:"type"`
	Priority            int                  `json:"priority"`
	IsPinned            bool                 `json:"is_pinned"`
	IsActive            bool                 `json:"is_active"`
	PublishStatus       domain.PublishStatus `json:"publish_status"`
	StartsAt            *string              `json:"starts_at,omitempty"`
	EndsAt              *string              `json:"ends_at,omitempty"`
	ExamTrackID         *string              `json:"exam_track_id,omitempty"`
	ExamDate            *string              `json:"exam_date,omitempty"`
	DaysBeforeStart     int                  `json:"days_before_start"`
	DaysLeft            *int                 `json:"days_left,omitempty"`
	ScheduleText        string               `json:"schedule_text,omitempty"`
	CTALabel            string               `json:"cta_label,omitempty"`
	CTAURL              string               `json:"cta_url,omitempty"`
	ExamTrack           *ExamTrackResponse   `json:"exam_track,omitempty"`
	RecommendedExamSets []ExamSetResponse    `json:"recommended_exam_sets"`
	CreatedBy           *string              `json:"created_by,omitempty"`
	UpdatedBy           *string              `json:"updated_by,omitempty"`
	CreatedAt           string               `json:"created_at"`
	UpdatedAt           string               `json:"updated_at"`
}

type AdminListResponse = pagination.PaginatedList[AnnouncementResponse]

func (uc *UseCase) ListAdmin(ctx context.Context, filter announcementrepo.AdminFilter) (*AdminListResponse, error) {
	items, total, err := uc.repository.ListAdmin(ctx, filter)
	if err != nil {
		return nil, err
	}
	responses := make([]AnnouncementResponse, len(items))
	for index := range items {
		responses[index] = toResponse(items[index], uc.now())
	}
	page, limit := pagination.Sanitize(filter.Page, filter.Limit)
	result := pagination.NewList(responses, page, limit, total)
	return &result, nil
}

func (uc *UseCase) GetAdmin(ctx context.Context, id uuid.UUID) (*AnnouncementResponse, error) {
	item, err := uc.repository.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, apperrors.ErrAnnouncementNotFound
	}
	response := toResponse(*item, uc.now())
	return &response, nil
}

func (uc *UseCase) Create(ctx context.Context, input MutationInput, actorID uuid.UUID) (*AnnouncementResponse, error) {
	item, err := uc.itemFromInput(ctx, input, nil)
	if err != nil {
		return nil, err
	}
	item.CreatedBy = &actorID
	item.UpdatedBy = &actorID
	if err := uc.repository.Create(ctx, item, input.ExamSetIDs); err != nil {
		return nil, err
	}
	uc.invalidate(ctx)
	return uc.GetAdmin(ctx, item.ID)
}

func (uc *UseCase) Update(ctx context.Context, id uuid.UUID, input MutationInput, actorID uuid.UUID) (*AnnouncementResponse, error) {
	existing, err := uc.repository.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, apperrors.ErrAnnouncementNotFound
	}
	item, err := uc.itemFromInput(ctx, input, &id)
	if err != nil {
		return nil, err
	}
	item.ID = id
	item.CreatedAt = existing.CreatedAt
	item.CreatedBy = existing.CreatedBy
	item.UpdatedBy = &actorID
	if err := uc.repository.Update(ctx, item, input.ExamSetIDs); err != nil {
		return nil, err
	}
	uc.invalidate(ctx)
	return uc.GetAdmin(ctx, id)
}

func (uc *UseCase) UpdateStatus(ctx context.Context, id uuid.UUID, input StatusInput, actorID uuid.UUID) (*AnnouncementResponse, error) {
	if !input.PublishStatus.Valid() {
		return nil, apperrors.ErrAnnouncementInvalidStatus
	}
	existing, err := uc.repository.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, apperrors.ErrAnnouncementNotFound
	}
	if input.PublishStatus == domain.StatusPublished {
		validationInput := inputFromAnnouncement(*existing)
		validationInput.PublishStatus = domain.StatusPublished
		if err := ValidateInput(validationInput); err != nil {
			return nil, err
		}
	}
	if err := uc.repository.UpdateStatus(ctx, id, input.PublishStatus, actorID); err != nil {
		return nil, err
	}
	uc.invalidate(ctx)
	return uc.GetAdmin(ctx, id)
}

func (uc *UseCase) Delete(ctx context.Context, id uuid.UUID) (*AnnouncementResponse, error) {
	existing, err := uc.GetAdmin(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := uc.repository.Delete(ctx, id); err != nil {
		return nil, err
	}
	uc.invalidate(ctx)
	return existing, nil
}

func (uc *UseCase) ListActive(ctx context.Context, announcementType string) ([]AnnouncementResponse, error) {
	if announcementType != "" && !domain.Type(announcementType).Valid() {
		return nil, apperrors.ValidationError("ประเภทประกาศไม่ถูกต้อง")
	}
	key := cache.AnnouncementsActive()
	if announcementType != "" {
		key = cache.AnnouncementsActiveByType(announcementType)
	}
	var cached []AnnouncementResponse
	if ok, _ := uc.cache.GetJSON(ctx, key, &cached); ok {
		return filterVisibleResponses(cached, uc.now()), nil
	}
	items, err := uc.repository.ListActiveCandidates(ctx, announcementType)
	if err != nil {
		return nil, err
	}
	responses := uc.visibleResponses(items)
	_ = uc.cache.SetJSON(ctx, key, responses, cache.TTLAnnouncements)
	_ = uc.cache.AddIndex(ctx, cache.IndexAnnouncements(), key, cache.TTLAnnouncements+cache.TTLIndexBuffer)
	return responses, nil
}

func (uc *UseCase) ListByTrack(ctx context.Context, trackCode string) ([]AnnouncementResponse, error) {
	key := cache.AnnouncementsByTrack(trackCode)
	var cached []AnnouncementResponse
	if ok, _ := uc.cache.GetJSON(ctx, key, &cached); ok {
		return filterVisibleResponses(cached, uc.now()), nil
	}
	items, err := uc.repository.ListTrackCandidates(ctx, trackCode)
	if err != nil {
		return nil, err
	}
	responses := uc.visibleResponses(items)
	_ = uc.cache.SetJSON(ctx, key, responses, cache.TTLAnnouncements)
	_ = uc.cache.AddIndex(ctx, cache.IndexAnnouncements(), key, cache.TTLAnnouncements+cache.TTLIndexBuffer)
	return responses, nil
}

func (uc *UseCase) GetPublic(ctx context.Context, slug string) (*AnnouncementResponse, error) {
	item, err := uc.repository.FindBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if item == nil || !item.VisibleAt(uc.now()) {
		return nil, apperrors.ErrAnnouncementNotFound
	}
	response := toResponse(*item, uc.now())
	return &response, nil
}

func (uc *UseCase) itemFromInput(ctx context.Context, input MutationInput, excludeID *uuid.UUID) (*domain.Announcement, error) {
	if input.PublishStatus == "" {
		input.PublishStatus = domain.StatusDraft
	}
	if err := ValidateInput(input); err != nil {
		return nil, err
	}
	taken, err := uc.repository.SlugExists(ctx, input.Slug, excludeID)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, apperrors.ErrAnnouncementSlugTaken
	}
	if input.ExamTrackID != nil {
		track, err := uc.tracks.FindByID(ctx, *input.ExamTrackID)
		if err != nil {
			return nil, err
		}
		if track == nil {
			return nil, apperrors.ErrExamTrackNotFound
		}
	}
	seenExamSetIDs := make(map[uuid.UUID]struct{}, len(input.ExamSetIDs))
	for _, examSetID := range input.ExamSetIDs {
		if _, duplicate := seenExamSetIDs[examSetID]; duplicate {
			return nil, apperrors.ValidationError("ชุดข้อสอบแนะนำซ้ำกัน")
		}
		seenExamSetIDs[examSetID] = struct{}{}
		examSet, err := uc.examSets.FindByID(ctx, examSetID)
		if err != nil {
			return nil, err
		}
		if examSet == nil {
			return nil, apperrors.ErrExamSetNotFound
		}
	}
	examDate, _ := parseExamDate(input.ExamDate)
	if input.Type != domain.TypeExamSchedule {
		examDate = nil
		input.DaysBeforeStart = 0
	}
	return &domain.Announcement{
		Title: input.Title, Slug: input.Slug, Summary: input.Summary, Content: input.Content,
		Type: input.Type, Priority: input.Priority, IsPinned: input.IsPinned, IsActive: input.IsActive,
		PublishStatus: input.PublishStatus, StartsAt: input.StartsAt, EndsAt: input.EndsAt,
		ExamTrackID: input.ExamTrackID, ExamDate: examDate, DaysBeforeStart: input.DaysBeforeStart,
		CTALabel: input.CTALabel, CTAURL: input.CTAURL,
	}, nil
}

func (uc *UseCase) visibleResponses(items []domain.Announcement) []AnnouncementResponse {
	now := uc.now()
	responses := make([]AnnouncementResponse, 0, len(items))
	for _, item := range items {
		if item.VisibleAt(now) {
			responses = append(responses, toResponse(item, now))
		}
	}
	return responses
}

func (uc *UseCase) invalidate(ctx context.Context) {
	if uc.invalidator != nil {
		uc.invalidator.OnAnnouncementChanged(ctx)
	}
}

func inputFromAnnouncement(item domain.Announcement) MutationInput {
	var examDate *string
	if item.ExamDate != nil {
		value := item.ExamDate.Format("2006-01-02")
		examDate = &value
	}
	return MutationInput{Title: item.Title, Slug: item.Slug, Summary: item.Summary, Content: item.Content,
		Type: item.Type, Priority: item.Priority, IsPinned: item.IsPinned, IsActive: item.IsActive,
		PublishStatus: item.PublishStatus, StartsAt: item.StartsAt, EndsAt: item.EndsAt,
		ExamTrackID: item.ExamTrackID, ExamDate: examDate, DaysBeforeStart: item.DaysBeforeStart,
		CTALabel: item.CTALabel, CTAURL: item.CTAURL}
}

func toResponse(item domain.Announcement, now time.Time) AnnouncementResponse {
	response := AnnouncementResponse{
		ID: item.ID.String(), Title: item.Title, Slug: item.Slug, Summary: item.Summary,
		Content: item.Content, Type: item.Type, Priority: item.Priority, IsPinned: item.IsPinned,
		IsActive: item.IsActive, PublishStatus: item.PublishStatus, DaysBeforeStart: item.DaysBeforeStart,
		DaysLeft: item.DaysLeft(now), ScheduleText: item.ScheduleText(now), CTALabel: item.CTALabel,
		CTAURL: item.CTAURL, RecommendedExamSets: make([]ExamSetResponse, 0, len(item.RecommendedSets)),
		CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339),
	}
	response.StartsAt = formatTime(item.StartsAt)
	response.EndsAt = formatTime(item.EndsAt)
	if item.ExamDate != nil {
		value := item.ExamDate.Format("2006-01-02")
		response.ExamDate = &value
	}
	response.ExamTrackID = formatUUID(item.ExamTrackID)
	response.CreatedBy = formatUUID(item.CreatedBy)
	response.UpdatedBy = formatUUID(item.UpdatedBy)
	if item.ExamTrack != nil {
		response.ExamTrack = &ExamTrackResponse{ID: item.ExamTrack.ID.String(), Code: item.ExamTrack.Code, Name: item.ExamTrack.Name}
		if response.CTAURL == "" {
			response.CTAURL = "/exams/" + item.ExamTrack.Code
		}
	}
	for _, examSet := range item.RecommendedSets {
		response.RecommendedExamSets = append(response.RecommendedExamSets, ExamSetResponse{ID: examSet.ID.String(), Code: examSet.Code, Title: examSet.Title})
	}
	return response
}

func formatTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}

func formatUUID(value *uuid.UUID) *string {
	if value == nil {
		return nil
	}
	formatted := value.String()
	return &formatted
}

func filterVisibleResponses(items []AnnouncementResponse, now time.Time) []AnnouncementResponse {
	visible := make([]AnnouncementResponse, 0, len(items))
	for _, item := range items {
		announcement := domain.Announcement{Type: item.Type, PublishStatus: item.PublishStatus, IsActive: item.IsActive, IsPinned: item.IsPinned, DaysBeforeStart: item.DaysBeforeStart}
		if item.StartsAt != nil {
			value, _ := time.Parse(time.RFC3339, *item.StartsAt)
			announcement.StartsAt = &value
		}
		if item.EndsAt != nil {
			value, _ := time.Parse(time.RFC3339, *item.EndsAt)
			announcement.EndsAt = &value
		}
		if item.ExamDate != nil {
			value, _ := time.Parse("2006-01-02", *item.ExamDate)
			announcement.ExamDate = &value
		}
		if announcement.VisibleAt(now) {
			item.DaysLeft = announcement.DaysLeft(now)
			item.ScheduleText = announcement.ScheduleText(now)
			visible = append(visible, item)
		}
	}
	return visible
}
