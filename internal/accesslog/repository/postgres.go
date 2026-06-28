package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"virtual-exam-api/internal/accesslog/domain"
	"virtual-exam-api/internal/common/pagination"
)

type AccessLogModel struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey"`
	UserID    *uuid.UUID `gorm:"type:uuid;index"`
	Email     *string    `gorm:"type:varchar(255);index"`
	EventType string     `gorm:"type:varchar(50);not null;index"`
	Success   bool       `gorm:"not null;default:true"`
	IPAddress *string    `gorm:"type:varchar(100)"`
	UserAgent *string    `gorm:"type:text"`
	Message   *string    `gorm:"type:text"`
	Metadata  []byte     `gorm:"type:jsonb"`
	CreatedAt time.Time  `gorm:"not null;index"`
}

func (AccessLogModel) TableName() string { return "access_logs" }

type AccessLogFilter struct {
	UserID    *uuid.UUID
	Email     string
	EventType string
	Success   *bool
	DateFrom  *time.Time
	DateTo    *time.Time
	Page      int
	Limit     int
	Sort      string
	Order     string
}

var accessLogSortColumns = map[string]string{
	"created_at": "created_at",
	"email":      "email",
	"event_type": "event_type",
	"success":    "success",
}

type Repository interface {
	Create(ctx context.Context, log *domain.AccessLog) error
	List(ctx context.Context, filter AccessLogFilter) ([]domain.AccessLog, int64, error)
	ListRecentByUserID(ctx context.Context, userID uuid.UUID, limit int) ([]domain.AccessLog, error)
}

type postgresRepository struct {
	db *gorm.DB
}

func NewPostgresRepository(db *gorm.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) Create(ctx context.Context, log *domain.AccessLog) error {
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

func (r *postgresRepository) List(ctx context.Context, filter AccessLogFilter) ([]domain.AccessLog, int64, error) {
	page, limit := pagination.Sanitize(filter.Page, filter.Limit)
	sortCol := pagination.ResolveSort(filter.Sort, accessLogSortColumns, "created_at")
	orderDir := pagination.ResolveOrder(filter.Order, true)
	q := r.db.WithContext(ctx).Model(&AccessLogModel{})
	if filter.UserID != nil {
		q = q.Where("user_id = ?", *filter.UserID)
	}
	if filter.Email != "" {
		q = q.Where("email ILIKE ?", "%"+filter.Email+"%")
	}
	if filter.EventType != "" {
		q = q.Where("event_type = ?", filter.EventType)
	}
	if filter.Success != nil {
		q = q.Where("success = ?", *filter.Success)
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
	var models []AccessLogModel
	if err := q.Order(pagination.OrderClause(sortCol, orderDir)).
		Offset(pagination.Offset(page, limit)).
		Limit(limit).
		Find(&models).Error; err != nil {
		return nil, 0, err
	}
	items := make([]domain.AccessLog, len(models))
	for i := range models {
		items[i] = toDomain(&models[i])
	}
	return items, total, nil
}

func (r *postgresRepository) ListRecentByUserID(ctx context.Context, userID uuid.UUID, limit int) ([]domain.AccessLog, error) {
	if limit < 1 {
		limit = 10
	}
	var models []AccessLogModel
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&models).Error; err != nil {
		return nil, err
	}
	items := make([]domain.AccessLog, len(models))
	for i := range models {
		items[i] = toDomain(&models[i])
	}
	return items, nil
}

func toModel(log *domain.AccessLog) (AccessLogModel, error) {
	var userID *uuid.UUID
	if log.UserID != nil {
		userID = log.UserID
	}
	var email *string
	if log.Email != "" {
		email = &log.Email
	}
	var ip *string
	if log.IPAddress != "" {
		ip = &log.IPAddress
	}
	var ua *string
	if log.UserAgent != "" {
		ua = &log.UserAgent
	}
	var msg *string
	if log.Message != "" {
		msg = &log.Message
	}
	var metadata []byte
	if log.Metadata != nil {
		b, err := json.Marshal(log.Metadata)
		if err != nil {
			return AccessLogModel{}, err
		}
		metadata = b
	}
	return AccessLogModel{
		ID:        log.ID,
		UserID:    userID,
		Email:     email,
		EventType: log.EventType,
		Success:   log.Success,
		IPAddress: ip,
		UserAgent: ua,
		Message:   msg,
		Metadata:  metadata,
		CreatedAt: log.CreatedAt,
	}, nil
}

func toDomain(m *AccessLogModel) domain.AccessLog {
	log := domain.AccessLog{
		ID:        m.ID,
		UserID:    m.UserID,
		EventType: m.EventType,
		Success:   m.Success,
		CreatedAt: m.CreatedAt,
	}
	if m.Email != nil {
		log.Email = *m.Email
	}
	if m.IPAddress != nil {
		log.IPAddress = *m.IPAddress
	}
	if m.UserAgent != nil {
		log.UserAgent = *m.UserAgent
	}
	if m.Message != nil {
		log.Message = *m.Message
	}
	if len(m.Metadata) > 0 {
		_ = json.Unmarshal(m.Metadata, &log.Metadata)
	}
	return log
}
