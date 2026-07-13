package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"virtual-exam-api/internal/common/pagination"
	"virtual-exam-api/internal/examset/domain"
)

type AdminFilter struct {
	Query       string
	TrackID     uuid.UUID
	AccessType  string
	Difficulty  string
	Mode        string
	Status      string
	IsActive    *bool
	Page        int
	Limit       int
	Sort        string
	Order       string
}

var examSetSortColumns = map[string]string{
	"created_at": "created_at",
	"updated_at": "updated_at",
	"title":      "title",
	"code":       "code",
	"status":     "status",
}

type AdminRepository interface {
	LifecycleStopOutbox
	List(ctx context.Context, filter AdminFilter) (pagination.PaginatedList[domain.ExamSet], error)
	Create(ctx context.Context, set *domain.ExamSet) error
	Update(ctx context.Context, set *domain.ExamSet) error
	Delete(ctx context.Context, id uuid.UUID) (deactivated bool, err error)
	UpdateTotalQuestions(ctx context.Context, examSetID uuid.UUID, count int) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status string, isActive bool) error
	UpdateIsActive(ctx context.Context, id uuid.UUID, isActive bool) error
	ListPendingLifecycleStops(ctx context.Context, examSetID uuid.UUID) ([]LifecycleStopEvent, error)
	MarkLifecycleStopDelivered(ctx context.Context, examSetID uuid.UUID, stoppedAt time.Time) error
}

type adminRepository struct {
	db *gorm.DB
}

func NewAdminRepository(db *gorm.DB) AdminRepository {
	return &adminRepository{db: db}
}

func (r *adminRepository) List(ctx context.Context, filter AdminFilter) (pagination.PaginatedList[domain.ExamSet], error) {
	page, limit := pagination.Sanitize(filter.Page, filter.Limit)
	sortCol := pagination.ResolveSort(filter.Sort, examSetSortColumns, "updated_at")
	orderDir := pagination.ResolveOrder(filter.Order, true)
	q := r.db.WithContext(ctx).Model(&ExamSetModel{}).Preload("ExamTrack")
	if filter.Query != "" {
		like := "%" + filter.Query + "%"
		q = q.Where("title ILIKE ? OR code ILIKE ? OR description ILIKE ?", like, like, like)
	}
	if filter.TrackID != uuid.Nil {
		q = q.Where("exam_track_id = ?", filter.TrackID)
	}
	if filter.AccessType != "" {
		q = q.Where("access_type = ?", filter.AccessType)
	}
	if filter.Difficulty != "" {
		q = q.Where("difficulty = ?", filter.Difficulty)
	}
	if filter.Mode != "" {
		q = q.Where("mode = ?", filter.Mode)
	}
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}
	if filter.IsActive != nil {
		q = q.Where("is_active = ?", *filter.IsActive)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return pagination.PaginatedList[domain.ExamSet]{}, err
	}
	var models []ExamSetModel
	err := q.Order(pagination.OrderClause(sortCol, orderDir)).Offset(pagination.Offset(page, limit)).Limit(limit).Find(&models).Error
	if err != nil {
		return pagination.PaginatedList[domain.ExamSet]{}, err
	}
	items := make([]domain.ExamSet, len(models))
	for i := range models {
		items[i] = toDomain(&models[i])
	}
	return pagination.NewList(items, page, limit, total), nil
}

func (r *adminRepository) Create(ctx context.Context, set *domain.ExamSet) error {
	if set.ID == uuid.Nil {
		set.ID = uuid.New()
	}
	now := time.Now().UTC()
	set.CreatedAt = now
	set.UpdatedAt = now
	if set.Currency == "" {
		set.Currency = "THB"
	}
	model := ExamSetModel{
		ID:              set.ID,
		ExamTrackID:     set.ExamTrackID,
		Code:            strings.ToLower(set.Code),
		Title:           set.Title,
		Description:     set.Description,
		CoverImageURL:   set.CoverImageURL,
		DurationMinutes: set.DurationMinutes,
		TotalQuestions:  set.TotalQuestions,
		PassingScore:    set.PassingScore,
		Difficulty:      set.Difficulty,
		AccessType:          set.AccessType,
		AllowSinglePurchase: set.AllowSinglePurchase,
		PriceAmount:         set.PriceAmount,
		OriginalPriceAmount: set.OriginalPriceAmount,
		Currency:            set.Currency,
		SalePriceAmount:     set.SalePriceAmount,
		Mode:            set.Mode,
		IsOfficial:      set.IsOfficial,
		IsFeatured:      set.IsFeatured,
		IsActive:        set.IsActive,
		Status:          domain.StatusDraft,
		AnswerSheetBlockColumns:      set.AnswerSheetLayout.BlockColumns,
		AnswerSheetQuestionsPerBlock: set.AnswerSheetLayout.QuestionsPerBlock,
		AnswerSheetChoiceLabelStyle:  set.AnswerSheetLayout.ChoiceLabelStyle,
		AnswerSheetShowHeader:        set.AnswerSheetLayout.ShowHeader,
		AnswerSheetShowInstructions:  set.AnswerSheetLayout.ShowInstructions,
		AnswerSheetShowCandidateInfo: set.AnswerSheetLayout.ShowCandidateInfo,
		CreatedAt:       set.CreatedAt,
		UpdatedAt:       set.UpdatedAt,
		PublishedAt:     set.PublishedAt,
	}
	return r.db.WithContext(ctx).Create(&model).Error
}

func (r *adminRepository) Update(ctx context.Context, set *domain.ExamSet) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		before, err := lockExamSetLifecycleState(tx, set.ID)
		if err != nil {
			return err
		}
		set.UpdatedAt = time.Now().UTC()
		updates := map[string]any{
			"exam_track_id":                    set.ExamTrackID,
			"code":                             strings.ToLower(set.Code),
			"title":                            set.Title,
			"description":                      set.Description,
			"cover_image_url":                  set.CoverImageURL,
			"duration_minutes":                 set.DurationMinutes,
			"total_questions":                  set.TotalQuestions,
			"passing_score":                    set.PassingScore,
			"difficulty":                       set.Difficulty,
			"access_type":                      set.AccessType,
			"allow_single_purchase":            set.AllowSinglePurchase,
			"price_amount":                     set.PriceAmount,
			"original_price_amount":            set.OriginalPriceAmount,
			"currency":                         set.Currency,
			"sale_price_amount":                set.SalePriceAmount,
			"mode":                             set.Mode,
			"is_official":                      set.IsOfficial,
			"is_featured":                      set.IsFeatured,
			"is_active":                        set.IsActive,
			"answer_sheet_block_columns":       set.AnswerSheetLayout.BlockColumns,
			"answer_sheet_questions_per_block": set.AnswerSheetLayout.QuestionsPerBlock,
			"answer_sheet_choice_label_style":  set.AnswerSheetLayout.ChoiceLabelStyle,
			"answer_sheet_show_header":         set.AnswerSheetLayout.ShowHeader,
			"answer_sheet_show_instructions":   set.AnswerSheetLayout.ShowInstructions,
			"answer_sheet_show_candidate_info": set.AnswerSheetLayout.ShowCandidateInfo,
			"updated_at":                       set.UpdatedAt,
		}
		if set.IsActive {
			updates["published_at"] = publicationActivationTimestamp(set.UpdatedAt)
		}
		if err := tx.Model(&ExamSetModel{}).Where("id = ?", set.ID).Updates(updates).Error; err != nil {
			return err
		}
		if isEligibleLifecycleState(before.Status, before.IsActive) && !isEligibleLifecycleState(before.Status, set.IsActive) {
			return insertLifecycleStopEvent(tx, set.ID, set.UpdatedAt)
		}
		return nil
	})
}

func (r *adminRepository) Delete(ctx context.Context, id uuid.UUID) (bool, error) {
	deactivated := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		before, err := lockExamSetLifecycleState(tx, id)
		if err != nil {
			return err
		}
		var hasImmutableReferences bool
		if err := tx.Raw(`
			SELECT EXISTS (SELECT 1 FROM exam_attempts WHERE exam_set_id = ?)
				OR EXISTS (SELECT 1 FROM leaderboard_season_exam_sets WHERE exam_set_id = ?)
				OR EXISTS (SELECT 1 FROM leaderboard_scores WHERE exam_set_id = ?)
				OR EXISTS (SELECT 1 FROM leaderboard_exam_set_stop_events WHERE exam_set_id = ?)
				OR EXISTS (SELECT 1 FROM exam_set_lifecycle_stop_events WHERE exam_set_id = ?)
		`, id, id, id, id, id).Scan(&hasImmutableReferences).Error; err != nil {
			return err
		}
		transitionAt := time.Now().UTC()
		if isEligibleLifecycleState(before.Status, before.IsActive) {
			if err := insertLifecycleStopEvent(tx, id, transitionAt); err != nil {
				return err
			}
		}
		if hasImmutableReferences || isEligibleLifecycleState(before.Status, before.IsActive) {
			deactivated = true
			return tx.Model(&ExamSetModel{}).Where("id = ?", id).Updates(map[string]any{
				"is_active":  false,
				"status":     domain.StatusArchived,
				"updated_at": transitionAt,
			}).Error
		}
		return tx.Delete(&ExamSetModel{}, "id = ?", id).Error
	})
	return deactivated, err
}

func (r *adminRepository) UpdateTotalQuestions(ctx context.Context, examSetID uuid.UUID, count int) error {
	return r.db.WithContext(ctx).Model(&ExamSetModel{}).Where("id = ?", examSetID).Updates(map[string]any{
		"total_questions": count,
		"updated_at":      time.Now().UTC(),
	}).Error
}

func (r *adminRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, isActive bool) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		before, err := lockExamSetLifecycleState(tx, id)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		updates := map[string]any{
			"status":     status,
			"is_active":  isActive,
			"updated_at": now,
		}
		if status == domain.StatusPublished && isActive {
			updates["published_at"] = gorm.Expr(`CASE
			WHEN status <> ? OR is_active = false OR published_at IS NULL THEN ?
			ELSE published_at
		END`, domain.StatusPublished, now)
		}
		if err := tx.Model(&ExamSetModel{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return err
		}
		if isEligibleLifecycleState(before.Status, before.IsActive) && !isEligibleLifecycleState(status, isActive) {
			return insertLifecycleStopEvent(tx, id, now)
		}
		return nil
	})
}

func (r *adminRepository) UpdateIsActive(ctx context.Context, id uuid.UUID, isActive bool) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		before, err := lockExamSetLifecycleState(tx, id)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		updates := map[string]any{
			"is_active":  isActive,
			"updated_at": now,
		}
		if isActive {
			updates["published_at"] = publicationActivationTimestamp(now)
		}
		if err := tx.Model(&ExamSetModel{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return err
		}
		if isEligibleLifecycleState(before.Status, before.IsActive) && !isEligibleLifecycleState(before.Status, isActive) {
			return insertLifecycleStopEvent(tx, id, now)
		}
		return nil
	})
}

func publicationActivationTimestamp(transitionAt time.Time) any {
	return gorm.Expr(`CASE
		WHEN status = ? AND (is_active = false OR published_at IS NULL) THEN ?
		ELSE published_at
	END`, domain.StatusPublished, transitionAt)
}

func normalizePagination(page, limit int) (int, int) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return page, limit
}

func IsValidSetCode(code string) bool {
	code = strings.ToLower(strings.TrimSpace(code))
	if len(code) < 2 || len(code) > 80 {
		return false
	}
	for _, ch := range code {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' {
			continue
		}
		return false
	}
	return true
}

func FindSetByIDAdmin(ctx context.Context, db *gorm.DB, id uuid.UUID) (*domain.ExamSet, error) {
	var model ExamSetModel
	err := db.WithContext(ctx).Preload("ExamTrack").Where("id = ?", id).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	set := toDomain(&model)
	return &set, nil
}

func FindSetByCodeAdmin(ctx context.Context, db *gorm.DB, code string) (*domain.ExamSet, error) {
	var model ExamSetModel
	err := db.WithContext(ctx).Preload("ExamTrack").Where("code = ?", strings.ToLower(code)).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	set := toDomain(&model)
	return &set, nil
}

func CountActiveSets(ctx context.Context, db *gorm.DB) (int64, error) {
	var count int64
	err := db.WithContext(ctx).Model(&ExamSetModel{}).
		Where("status = ? AND is_active = ?", domain.StatusPublished, true).
		Count(&count).Error
	return count, err
}

func CountPremiumSets(ctx context.Context, db *gorm.DB) (int64, error) {
	var count int64
	err := db.WithContext(ctx).Model(&ExamSetModel{}).Where("access_type = ?", domain.AccessPremium).Count(&count).Error
	return count, err
}

func CountFreeSets(ctx context.Context, db *gorm.DB) (int64, error) {
	var count int64
	err := db.WithContext(ctx).Model(&ExamSetModel{}).Where("access_type = ?", domain.AccessFree).Count(&count).Error
	return count, err
}

func ListLatestSets(ctx context.Context, db *gorm.DB, limit int) ([]domain.ExamSet, error) {
	if limit < 1 {
		limit = 5
	}
	var models []ExamSetModel
	err := db.WithContext(ctx).Preload("ExamTrack").Order("created_at DESC").Limit(limit).Find(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]domain.ExamSet, len(models))
	for i := range models {
		out[i] = toDomain(&models[i])
	}
	return out, nil
}
