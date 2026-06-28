package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"virtual-exam-api/internal/auditlog/domain"
	"virtual-exam-api/internal/common/pagination"
)

type AuditLogModel struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey"`
	ActorUserID  *uuid.UUID `gorm:"type:uuid;index"`
	ActorEmail   *string    `gorm:"type:varchar(255)"`
	Action       string     `gorm:"type:varchar(100);not null;index"`
	ResourceType string     `gorm:"type:varchar(100);not null;index"`
	ResourceID   *uuid.UUID `gorm:"type:uuid;index"`
	ResourceName *string    `gorm:"type:text"`
	BeforeData   []byte     `gorm:"type:jsonb"`
	AfterData    []byte     `gorm:"type:jsonb"`
	IPAddress    *string    `gorm:"type:varchar(100)"`
	UserAgent    *string    `gorm:"type:text"`
	Metadata     []byte     `gorm:"type:jsonb"`
	CreatedAt    time.Time  `gorm:"not null;index"`
}

func (AuditLogModel) TableName() string { return "audit_logs" }

type AuditLogFilter struct {
	ActorUserID  *uuid.UUID
	Action       string
	ResourceType string
	ResourceID   *uuid.UUID
	DateFrom     *time.Time
	DateTo       *time.Time
	Page         int
	Limit        int
	Sort         string
	Order        string
}

var auditLogSortColumns = map[string]string{
	"created_at":    "created_at",
	"action":        "action",
	"resource_type": "resource_type",
}

type Repository interface {
	Create(ctx context.Context, log *domain.AuditLog) error
	List(ctx context.Context, filter AuditLogFilter) ([]domain.AuditLog, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.AuditLog, error)
	ListRecentByResource(ctx context.Context, resourceType string, resourceID uuid.UUID, limit int) ([]domain.AuditLog, error)
	ListRecentByActorUserID(ctx context.Context, actorUserID uuid.UUID, limit int) ([]domain.AuditLog, error)
}

type postgresRepository struct {
	db *gorm.DB
}

func NewPostgresRepository(db *gorm.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) Create(ctx context.Context, log *domain.AuditLog) error {
	model, err := toModel(log)
	if err != nil {
		return err
	}
	if model.ID == uuid.Nil {
		model.ID = uuid.New()
	}
	if model.CreatedAt.IsZero() {
		model.CreatedAt = time.Now().UTC()
	}
	return r.db.WithContext(ctx).Create(&model).Error
}

func (r *postgresRepository) List(ctx context.Context, filter AuditLogFilter) ([]domain.AuditLog, int64, error) {
	page, limit := pagination.Sanitize(filter.Page, filter.Limit)
	sortCol := pagination.ResolveSort(filter.Sort, auditLogSortColumns, "created_at")
	orderDir := pagination.ResolveOrder(filter.Order, true)
	q := r.db.WithContext(ctx).Model(&AuditLogModel{})
	if filter.ActorUserID != nil {
		q = q.Where("actor_user_id = ?", *filter.ActorUserID)
	}
	if filter.Action != "" {
		q = q.Where("action = ?", filter.Action)
	}
	if filter.ResourceType != "" {
		q = q.Where("resource_type = ?", filter.ResourceType)
	}
	if filter.ResourceID != nil {
		q = q.Where("resource_id = ?", *filter.ResourceID)
	}
	if filter.DateFrom != nil {
		q = q.Where("created_at >= ?", *filter.DateFrom)
	}
	if filter.DateTo != nil {
		q = q.Where("created_at <= ?", *filter.DateTo)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var models []AuditLogModel
	if err := q.Order(pagination.OrderClause(sortCol, orderDir)).
		Offset(pagination.Offset(page, limit)).
		Limit(limit).
		Find(&models).Error; err != nil {
		return nil, 0, err
	}
	items := make([]domain.AuditLog, len(models))
	for i := range models {
		items[i] = toDomain(&models[i])
	}
	return items, total, nil
}

func (r *postgresRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.AuditLog, error) {
	var model AuditLogModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&model).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	log := toDomain(&model)
	return &log, nil
}

func (r *postgresRepository) ListRecentByResource(ctx context.Context, resourceType string, resourceID uuid.UUID, limit int) ([]domain.AuditLog, error) {
	if limit < 1 {
		limit = 10
	}
	var models []AuditLogModel
	if err := r.db.WithContext(ctx).
		Where("resource_type = ? AND resource_id = ?", resourceType, resourceID).
		Order("created_at DESC").
		Limit(limit).
		Find(&models).Error; err != nil {
		return nil, err
	}
	items := make([]domain.AuditLog, len(models))
	for i := range models {
		items[i] = toDomain(&models[i])
	}
	return items, nil
}

func (r *postgresRepository) ListRecentByActorUserID(ctx context.Context, actorUserID uuid.UUID, limit int) ([]domain.AuditLog, error) {
	if limit < 1 {
		limit = 10
	}
	var models []AuditLogModel
	if err := r.db.WithContext(ctx).
		Where("resource_type = ? AND resource_id = ?", "user", actorUserID).
		Order("created_at DESC").
		Limit(limit).
		Find(&models).Error; err != nil {
		return nil, err
	}
	items := make([]domain.AuditLog, len(models))
	for i := range models {
		items[i] = toDomain(&models[i])
	}
	return items, nil
}

func toModel(log *domain.AuditLog) (AuditLogModel, error) {
	var actorID *uuid.UUID
	if log.ActorUserID != nil {
		actorID = log.ActorUserID
	}
	var actorEmail *string
	if log.ActorEmail != "" {
		actorEmail = &log.ActorEmail
	}
	var resourceID *uuid.UUID
	if log.ResourceID != nil {
		resourceID = log.ResourceID
	}
	var resourceName *string
	if log.ResourceName != "" {
		resourceName = &log.ResourceName
	}
	var ip *string
	if log.IPAddress != "" {
		ip = &log.IPAddress
	}
	var ua *string
	if log.UserAgent != "" {
		ua = &log.UserAgent
	}
	before, err := marshalJSON(log.BeforeData)
	if err != nil {
		return AuditLogModel{}, err
	}
	after, err := marshalJSON(log.AfterData)
	if err != nil {
		return AuditLogModel{}, err
	}
	metadata, err := marshalJSON(log.Metadata)
	if err != nil {
		return AuditLogModel{}, err
	}
	return AuditLogModel{
		ID:           log.ID,
		ActorUserID:  actorID,
		ActorEmail:   actorEmail,
		Action:       log.Action,
		ResourceType: log.ResourceType,
		ResourceID:   resourceID,
		ResourceName: resourceName,
		BeforeData:   before,
		AfterData:    after,
		IPAddress:    ip,
		UserAgent:    ua,
		Metadata:     metadata,
		CreatedAt:    log.CreatedAt,
	}, nil
}

func marshalJSON(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

func toDomain(m *AuditLogModel) domain.AuditLog {
	log := domain.AuditLog{
		ID:           m.ID,
		ActorUserID:  m.ActorUserID,
		Action:       m.Action,
		ResourceType: m.ResourceType,
		ResourceID:   m.ResourceID,
		CreatedAt:    m.CreatedAt,
	}
	if m.ActorEmail != nil {
		log.ActorEmail = *m.ActorEmail
	}
	if m.ResourceName != nil {
		log.ResourceName = *m.ResourceName
	}
	if m.IPAddress != nil {
		log.IPAddress = *m.IPAddress
	}
	if m.UserAgent != nil {
		log.UserAgent = *m.UserAgent
	}
	if len(m.BeforeData) > 0 {
		_ = json.Unmarshal(m.BeforeData, &log.BeforeData)
	}
	if len(m.AfterData) > 0 {
		_ = json.Unmarshal(m.AfterData, &log.AfterData)
	}
	if len(m.Metadata) > 0 {
		_ = json.Unmarshal(m.Metadata, &log.Metadata)
	}
	return log
}
