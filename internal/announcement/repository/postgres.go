package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"virtual-exam-api/internal/announcement/domain"
	"virtual-exam-api/internal/common/pagination"
)

type AnnouncementModel struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey"`
	Title           string    `gorm:"not null"`
	Slug            string    `gorm:"not null"`
	Summary         string
	Content         string
	Type            string `gorm:"not null"`
	Priority        int    `gorm:"not null;default:0"`
	IsPinned        bool   `gorm:"not null;default:false"`
	IsActive        bool   `gorm:"not null;default:true"`
	PublishStatus   string `gorm:"not null;default:draft"`
	StartsAt        *time.Time
	EndsAt          *time.Time
	ExamTrackID     *uuid.UUID `gorm:"type:uuid"`
	ExamDate        *time.Time `gorm:"type:date"`
	DaysBeforeStart int        `gorm:"not null;default:0"`
	CTALabel        string
	CTAURL          string
	CreatedBy       *uuid.UUID `gorm:"type:uuid"`
	UpdatedBy       *uuid.UUID `gorm:"type:uuid"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (AnnouncementModel) TableName() string { return "announcements" }

type AnnouncementExamSetModel struct {
	AnnouncementID uuid.UUID `gorm:"type:uuid;primaryKey"`
	ExamSetID      uuid.UUID `gorm:"type:uuid;primaryKey"`
	SortOrder      int       `gorm:"not null"`
}

func (AnnouncementExamSetModel) TableName() string { return "announcement_exam_sets" }

type AdminFilter struct {
	Query         string
	Type          string
	PublishStatus string
	IsActive      *bool
	Page          int
	Limit         int
	Sort          string
	Order         string
}

type Repository interface {
	ListAdmin(ctx context.Context, filter AdminFilter) ([]domain.Announcement, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Announcement, error)
	FindBySlug(ctx context.Context, slug string) (*domain.Announcement, error)
	SlugExists(ctx context.Context, slug string, excludeID *uuid.UUID) (bool, error)
	Create(ctx context.Context, announcement *domain.Announcement, examSetIDs []uuid.UUID) error
	Update(ctx context.Context, announcement *domain.Announcement, examSetIDs []uuid.UUID) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.PublishStatus, actorID uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListActiveCandidates(ctx context.Context, announcementType string) ([]domain.Announcement, error)
	ListTrackCandidates(ctx context.Context, trackCode string) ([]domain.Announcement, error)
}

type postgresRepository struct {
	db *gorm.DB
}

func NewPostgresRepository(db *gorm.DB) Repository {
	return &postgresRepository{db: db}
}

var sortColumns = map[string]string{
	"created_at":     "created_at",
	"updated_at":     "updated_at",
	"title":          "title",
	"type":           "type",
	"publish_status": "publish_status",
	"exam_date":      "exam_date",
}

func (r *postgresRepository) ListAdmin(ctx context.Context, filter AdminFilter) ([]domain.Announcement, int64, error) {
	page, limit := pagination.Sanitize(filter.Page, filter.Limit)
	q := r.db.WithContext(ctx).Model(&AnnouncementModel{})
	if filter.Query != "" {
		like := "%" + filter.Query + "%"
		q = q.Where("title ILIKE ? OR slug ILIKE ? OR summary ILIKE ?", like, like, like)
	}
	if filter.Type != "" {
		q = q.Where("type = ?", filter.Type)
	}
	if filter.PublishStatus != "" {
		q = q.Where("publish_status = ?", filter.PublishStatus)
	}
	if filter.IsActive != nil {
		q = q.Where("is_active = ?", *filter.IsActive)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var models []AnnouncementModel
	sortColumn := pagination.ResolveSort(filter.Sort, sortColumns, "updated_at")
	order := pagination.ResolveOrder(filter.Order, true)
	if err := q.Order(pagination.OrderClause(sortColumn, order)).Offset(pagination.Offset(page, limit)).Limit(limit).Find(&models).Error; err != nil {
		return nil, 0, err
	}
	items := modelsToDomain(models)
	if err := r.hydrate(ctx, items, false); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *postgresRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Announcement, error) {
	var model AnnouncementModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	item := modelToDomain(model)
	items := []domain.Announcement{item}
	if err := r.hydrate(ctx, items, false); err != nil {
		return nil, err
	}
	return &items[0], nil
}

func (r *postgresRepository) FindBySlug(ctx context.Context, slug string) (*domain.Announcement, error) {
	var model AnnouncementModel
	err := r.db.WithContext(ctx).Where("LOWER(slug) = LOWER(?)", slug).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	item := modelToDomain(model)
	items := []domain.Announcement{item}
	if err := r.hydrate(ctx, items, true); err != nil {
		return nil, err
	}
	return &items[0], nil
}

func (r *postgresRepository) SlugExists(ctx context.Context, slug string, excludeID *uuid.UUID) (bool, error) {
	q := r.db.WithContext(ctx).Model(&AnnouncementModel{}).Where("LOWER(slug) = LOWER(?)", slug)
	if excludeID != nil {
		q = q.Where("id <> ?", *excludeID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *postgresRepository) Create(ctx context.Context, announcement *domain.Announcement, examSetIDs []uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if announcement.ID == uuid.Nil {
			announcement.ID = uuid.New()
		}
		now := time.Now().UTC()
		announcement.CreatedAt = now
		announcement.UpdatedAt = now
		model := domainToModel(*announcement)
		if err := tx.Create(&model).Error; err != nil {
			return err
		}
		return replaceExamSets(tx, announcement.ID, examSetIDs)
	})
}

func (r *postgresRepository) Update(ctx context.Context, announcement *domain.Announcement, examSetIDs []uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		announcement.UpdatedAt = time.Now().UTC()
		updates := map[string]any{
			"title": announcement.Title, "slug": strings.ToLower(announcement.Slug),
			"summary": announcement.Summary, "content": announcement.Content,
			"type": announcement.Type, "priority": announcement.Priority,
			"is_pinned": announcement.IsPinned, "is_active": announcement.IsActive,
			"publish_status": announcement.PublishStatus, "starts_at": announcement.StartsAt,
			"ends_at": announcement.EndsAt, "exam_track_id": announcement.ExamTrackID,
			"exam_date": announcement.ExamDate, "days_before_start": announcement.DaysBeforeStart,
			"cta_label": announcement.CTALabel, "cta_url": announcement.CTAURL,
			"updated_by": announcement.UpdatedBy, "updated_at": announcement.UpdatedAt,
		}
		if err := tx.Model(&AnnouncementModel{}).Where("id = ?", announcement.ID).Updates(updates).Error; err != nil {
			return err
		}
		return replaceExamSets(tx, announcement.ID, examSetIDs)
	})
}

func (r *postgresRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.PublishStatus, actorID uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&AnnouncementModel{}).Where("id = ?", id).Updates(map[string]any{
		"publish_status": status,
		"updated_by":     actorID,
		"updated_at":     time.Now().UTC(),
	}).Error
}

func (r *postgresRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&AnnouncementModel{}, "id = ?", id).Error
}

func (r *postgresRepository) ListActiveCandidates(ctx context.Context, announcementType string) ([]domain.Announcement, error) {
	now := time.Now().UTC()
	q := r.db.WithContext(ctx).Where("publish_status = ? AND is_active = ?", domain.StatusPublished, true).
		Where("(starts_at IS NULL OR starts_at <= ?) AND (ends_at IS NULL OR ends_at >= ?)", now, now)
	if announcementType != "" {
		q = q.Where("type = ?", announcementType)
	}
	var models []AnnouncementModel
	if err := q.Order("is_pinned DESC, priority DESC, updated_at DESC").Find(&models).Error; err != nil {
		return nil, err
	}
	items := modelsToDomain(models)
	if err := r.hydrate(ctx, items, true); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *postgresRepository) ListTrackCandidates(ctx context.Context, trackCode string) ([]domain.Announcement, error) {
	now := time.Now().UTC()
	var models []AnnouncementModel
	err := r.db.WithContext(ctx).Table("announcements a").
		Select("a.*").
		Joins("JOIN exam_tracks t ON t.id = a.exam_track_id").
		Where("t.code = ? AND a.publish_status = ? AND a.is_active = ?", trackCode, domain.StatusPublished, true).
		Where("(a.starts_at IS NULL OR a.starts_at <= ?) AND (a.ends_at IS NULL OR a.ends_at >= ?)", now, now).
		Order("a.is_pinned DESC, a.priority DESC, a.updated_at DESC").
		Scan(&models).Error
	if err != nil {
		return nil, err
	}
	items := modelsToDomain(models)
	if err := r.hydrate(ctx, items, true); err != nil {
		return nil, err
	}
	return items, nil
}

func replaceExamSets(tx *gorm.DB, announcementID uuid.UUID, examSetIDs []uuid.UUID) error {
	if err := tx.Where("announcement_id = ?", announcementID).Delete(&AnnouncementExamSetModel{}).Error; err != nil {
		return err
	}
	rows := make([]AnnouncementExamSetModel, len(examSetIDs))
	for index, examSetID := range examSetIDs {
		rows[index] = AnnouncementExamSetModel{AnnouncementID: announcementID, ExamSetID: examSetID, SortOrder: index}
	}
	if len(rows) == 0 {
		return nil
	}
	return tx.Create(&rows).Error
}

type trackRow struct {
	AnnouncementID uuid.UUID
	ID             uuid.UUID
	Code           string
	Name           string
}

type examSetRow struct {
	AnnouncementID uuid.UUID
	ID             uuid.UUID
	Code           string
	Title          string
	SortOrder      int
}

func (r *postgresRepository) hydrate(ctx context.Context, items []domain.Announcement, publicExamSets bool) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(items))
	positions := make(map[uuid.UUID]int, len(items))
	for index := range items {
		ids[index] = items[index].ID
		positions[items[index].ID] = index
	}

	var tracks []trackRow
	if err := r.db.WithContext(ctx).Table("announcements a").
		Select("a.id AS announcement_id, t.id, t.code, t.name").
		Joins("JOIN exam_tracks t ON t.id = a.exam_track_id").
		Where("a.id IN ?", ids).Scan(&tracks).Error; err != nil {
		return err
	}
	for _, row := range tracks {
		index := positions[row.AnnouncementID]
		items[index].ExamTrack = &domain.ExamTrackSummary{ID: row.ID, Code: row.Code, Name: row.Name}
	}

	q := r.db.WithContext(ctx).Table("announcement_exam_sets aes").
		Select("aes.announcement_id, es.id, es.code, es.title, aes.sort_order").
		Joins("JOIN exam_sets es ON es.id = aes.exam_set_id").Where("aes.announcement_id IN ?", ids)
	if publicExamSets {
		q = q.Where("es.status = ? AND es.is_active = ?", "published", true)
	}
	var sets []examSetRow
	if err := q.Order("aes.announcement_id, aes.sort_order").Scan(&sets).Error; err != nil {
		return err
	}
	for _, row := range sets {
		index := positions[row.AnnouncementID]
		items[index].RecommendedSets = append(items[index].RecommendedSets, domain.ExamSetSummary{
			ID: row.ID, Code: row.Code, Title: row.Title, SortOrder: row.SortOrder,
		})
	}
	return nil
}

func modelsToDomain(models []AnnouncementModel) []domain.Announcement {
	items := make([]domain.Announcement, len(models))
	for index, model := range models {
		items[index] = modelToDomain(model)
	}
	return items
}

func modelToDomain(model AnnouncementModel) domain.Announcement {
	return domain.Announcement{
		ID: model.ID, Title: model.Title, Slug: model.Slug, Summary: model.Summary,
		Content: model.Content, Type: domain.Type(model.Type), Priority: model.Priority,
		IsPinned: model.IsPinned, IsActive: model.IsActive,
		PublishStatus: domain.PublishStatus(model.PublishStatus), StartsAt: model.StartsAt,
		EndsAt: model.EndsAt, ExamTrackID: model.ExamTrackID, ExamDate: model.ExamDate,
		DaysBeforeStart: model.DaysBeforeStart, CTALabel: model.CTALabel, CTAURL: model.CTAURL,
		CreatedBy: model.CreatedBy, UpdatedBy: model.UpdatedBy,
		CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt,
	}
}

func domainToModel(item domain.Announcement) AnnouncementModel {
	return AnnouncementModel{
		ID: item.ID, Title: item.Title, Slug: strings.ToLower(item.Slug), Summary: item.Summary,
		Content: item.Content, Type: string(item.Type), Priority: item.Priority,
		IsPinned: item.IsPinned, IsActive: item.IsActive, PublishStatus: string(item.PublishStatus),
		StartsAt: item.StartsAt, EndsAt: item.EndsAt, ExamTrackID: item.ExamTrackID,
		ExamDate: item.ExamDate, DaysBeforeStart: item.DaysBeforeStart,
		CTALabel: item.CTALabel, CTAURL: item.CTAURL, CreatedBy: item.CreatedBy,
		UpdatedBy: item.UpdatedBy, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}
