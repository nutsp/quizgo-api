package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	entdomain "virtual-exam-api/internal/entitlement/domain"
	entrepo "virtual-exam-api/internal/entitlement/repository"
	"virtual-exam-api/internal/payment/domain"
)

type PaymentRequestModel struct {
	ID                uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID            uuid.UUID `gorm:"type:uuid;not null;index"`
	PackageID         string    `gorm:"type:varchar(80);not null;index"`
	PackageName       string    `gorm:"type:varchar(160);not null"`
	DurationMonths    int       `gorm:"not null"`
	OriginalPrice     int       `gorm:"not null"`
	SalePrice         int       `gorm:"not null"`
	DiscountPercent   int       `gorm:"not null"`
	Currency          string    `gorm:"type:varchar(3);not null"`
	Status            string    `gorm:"type:varchar(40);not null;index"`
	Provider          string    `gorm:"type:varchar(40);not null"`
	ProviderReference string    `gorm:"type:varchar(160);not null;index"`
	QRImageURL        string    `gorm:"type:text;not null"`
	QRExpiresAt       time.Time `gorm:"not null;index"`
	ProofStorageKey   *string   `gorm:"type:text"`
	ProofOriginalName *string   `gorm:"type:text"`
	ProofMIMEType     *string   `gorm:"type:varchar(100)"`
	ProofSize         int64
	ProofUploadedAt   *time.Time
	ReviewedBy        *uuid.UUID `gorm:"type:uuid"`
	ReviewedAt        *time.Time
	RejectionReason   *string    `gorm:"type:text"`
	EntitlementID     *uuid.UUID `gorm:"type:uuid"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (PaymentRequestModel) TableName() string { return "payment_requests" }

type PostgresRepository struct{ db *gorm.DB }

func NewPostgresRepository(db *gorm.DB) *PostgresRepository { return &PostgresRepository{db: db} }

func (r *PostgresRepository) Create(ctx context.Context, payment *domain.PaymentRequest) error {
	model := toModel(payment)
	return r.db.WithContext(ctx).Create(&model).Error
}

func (r *PostgresRepository) FindReusable(ctx context.Context, userID uuid.UUID, packageID string, now time.Time) (*domain.PaymentRequest, error) {
	var model PaymentRequestModel
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND package_id = ? AND status = ? AND qr_expires_at > ?", userID, packageID, domain.StatusAwaitingProof, now).
		Order("created_at DESC").First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toDomain(model), nil
}

func (r *PostgresRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.PaymentRequest, error) {
	var row paymentRow
	err := r.baseQuery(r.db.WithContext(ctx)).Where("payment_requests.id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return row.toDomain(), nil
}

func (r *PostgresRepository) AttachProof(ctx context.Context, id uuid.UUID, proof domain.Proof) error {
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).Model(&PaymentRequestModel{}).
		Where("id = ? AND status = ?", id, domain.StatusAwaitingProof).
		Updates(map[string]any{
			"proof_storage_key": proof.StorageKey, "proof_original_name": proof.OriginalName,
			"proof_mime_type": proof.MIMEType, "proof_size": proof.Size,
			"proof_uploaded_at": now, "status": domain.StatusPendingReview, "updated_at": now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return domain.ErrStateConflict
	}
	return nil
}

type paymentRow struct {
	PaymentRequestModel
	UserDisplayName string `gorm:"column:user_display_name"`
	UserEmail       string `gorm:"column:user_email"`
}

func (r *PostgresRepository) baseQuery(db *gorm.DB) *gorm.DB {
	return db.Table("payment_requests").
		Select("payment_requests.*, users.display_name AS user_display_name, users.email AS user_email").
		Joins("JOIN users ON users.id = payment_requests.user_id")
}

func (r *PostgresRepository) List(ctx context.Context, params domain.ListParams) ([]domain.PaymentRequest, int64, error) {
	query := r.baseQuery(r.db.WithContext(ctx))
	if params.Status != "" {
		query = query.Where("payment_requests.status = ?", params.Status)
	}
	if q := strings.TrimSpace(params.Query); q != "" {
		like := "%" + strings.ToLower(q) + "%"
		query = query.Where("LOWER(users.display_name) LIKE ? OR LOWER(users.email) LIKE ? OR LOWER(payment_requests.provider_reference) LIKE ?", like, like, like)
	}
	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []paymentRow
	if err := query.Order("payment_requests.created_at DESC").Offset((params.Page - 1) * params.Limit).Limit(params.Limit).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	items := make([]domain.PaymentRequest, len(rows))
	for i := range rows {
		items[i] = *rows[i].toDomain()
	}
	return items, total, nil
}

func (r *PostgresRepository) Approve(ctx context.Context, id, reviewerID uuid.UUID, now time.Time) (*domain.PaymentRequest, error) {
	var approvedID uuid.UUID
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var payment PaymentRequestModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&payment, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrNotFound
			}
			return err
		}
		if payment.Status != domain.StatusPendingReview {
			return domain.ErrStateConflict
		}

		var existingExpiry *time.Time
		var active entrepo.EntitlementModel
		err := tx.Where("user_id = ? AND entitlement_type = ? AND is_active = TRUE AND expires_at > ?", payment.UserID, entdomain.TypePremium, now).
			Order("expires_at DESC").First(&active).Error
		if err == nil {
			existingExpiry = active.ExpiresAt
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		startsAt, expiresAt := nextEntitlementWindow(now, existingExpiry, payment.DurationMonths)
		note := "Payment " + payment.ID.String()
		entitlement := entrepo.EntitlementModel{
			ID: uuid.New(), UserID: payment.UserID, EntitlementType: entdomain.TypePremium,
			Source: entdomain.SourcePurchase, StartsAt: startsAt, ExpiresAt: &expiresAt,
			IsActive: true, Notes: &note, GrantedBy: &reviewerID, CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.Create(&entitlement).Error; err != nil {
			return err
		}
		approvedID = entitlement.ID
		result := tx.Model(&PaymentRequestModel{}).Where("id = ? AND status = ?", id, domain.StatusPendingReview).
			Updates(map[string]any{"status": domain.StatusApproved, "reviewed_by": reviewerID, "reviewed_at": now, "entitlement_id": entitlement.ID, "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrStateConflict
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	payment, err := r.FindByID(ctx, id)
	if payment != nil {
		payment.EntitlementID = &approvedID
	}
	return payment, err
}

func (r *PostgresRepository) Reject(ctx context.Context, id, reviewerID uuid.UUID, reason string, now time.Time) (*domain.PaymentRequest, error) {
	result := r.db.WithContext(ctx).Model(&PaymentRequestModel{}).Where("id = ? AND status = ?", id, domain.StatusPendingReview).
		Updates(map[string]any{"status": domain.StatusRejected, "reviewed_by": reviewerID, "reviewed_at": now, "rejection_reason": reason, "updated_at": now})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, domain.ErrStateConflict
	}
	return r.FindByID(ctx, id)
}

func nextEntitlementWindow(now time.Time, existingExpiry *time.Time, months int) (time.Time, time.Time) {
	starts := now
	if existingExpiry != nil && existingExpiry.After(now) {
		starts = *existingExpiry
	}
	return starts, starts.AddDate(0, months, 0)
}

func toModel(p *domain.PaymentRequest) PaymentRequestModel {
	return PaymentRequestModel{
		ID: p.ID, UserID: p.UserID, PackageID: p.PackageID, PackageName: p.PackageName,
		DurationMonths: p.DurationMonths, OriginalPrice: p.OriginalPrice, SalePrice: p.SalePrice,
		DiscountPercent: p.DiscountPercent, Currency: p.Currency, Status: p.Status,
		Provider: p.Provider, ProviderReference: p.ProviderReference, QRImageURL: p.QRImageURL,
		QRExpiresAt: p.QRExpiresAt, ProofStorageKey: p.ProofStorageKey, ProofOriginalName: p.ProofOriginalName,
		ProofMIMEType: p.ProofMIMEType, ProofSize: p.ProofSize, ProofUploadedAt: p.ProofUploadedAt,
		ReviewedBy: p.ReviewedBy, ReviewedAt: p.ReviewedAt, RejectionReason: p.RejectionReason,
		EntitlementID: p.EntitlementID, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func toDomain(m PaymentRequestModel) *domain.PaymentRequest {
	return &domain.PaymentRequest{
		ID: m.ID, UserID: m.UserID, PackageID: m.PackageID, PackageName: m.PackageName,
		DurationMonths: m.DurationMonths, OriginalPrice: m.OriginalPrice, SalePrice: m.SalePrice,
		DiscountPercent: m.DiscountPercent, Currency: m.Currency, Status: m.Status,
		Provider: m.Provider, ProviderReference: m.ProviderReference, QRImageURL: m.QRImageURL,
		QRExpiresAt: m.QRExpiresAt, ProofStorageKey: m.ProofStorageKey, ProofOriginalName: m.ProofOriginalName,
		ProofMIMEType: m.ProofMIMEType, ProofSize: m.ProofSize, ProofUploadedAt: m.ProofUploadedAt,
		ReviewedBy: m.ReviewedBy, ReviewedAt: m.ReviewedAt, RejectionReason: m.RejectionReason,
		EntitlementID: m.EntitlementID, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

func (r paymentRow) toDomain() *domain.PaymentRequest {
	payment := toDomain(r.PaymentRequestModel)
	payment.UserDisplayName = r.UserDisplayName
	payment.UserEmail = r.UserEmail
	return payment
}
