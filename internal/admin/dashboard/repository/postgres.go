package repository

import (
	"context"
	"log"
	"time"

	"gorm.io/gorm"
	entitlementdomain "virtual-exam-api/internal/entitlement/domain"
	userrepo "virtual-exam-api/internal/user/repository"
)

type AdminUserMetrics struct {
	TotalUsers     int64 `json:"total_users"`
	NewUsersToday  int64 `json:"new_users_today"`
	NewUsers7D     int64 `json:"new_users_7d"`
	ActiveUsers7D  int64 `json:"active_users_7d"`
	PremiumUsers   int64 `json:"premium_users"`
	SuspendedUsers int64 `json:"suspended_users"`
}

type Repository interface {
	GetAdminUserMetrics(ctx context.Context) (*AdminUserMetrics, error)
	GetAdminCharts(ctx context.Context) AdminCharts
}

type postgresRepository struct {
	db *gorm.DB
}

func NewPostgresRepository(db *gorm.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) GetAdminUserMetrics(ctx context.Context) (*AdminUserMetrics, error) {
	metrics := &AdminUserMetrics{}
	now := time.Now()
	sevenDaysAgo := now.AddDate(0, 0, -7)
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	if err := r.db.WithContext(ctx).Model(&userrepo.UserModel{}).Count(&metrics.TotalUsers).Error; err != nil {
		return nil, err
	}

	if err := r.db.WithContext(ctx).Model(&userrepo.UserModel{}).
		Where("created_at >= ?", startOfDay).
		Count(&metrics.NewUsersToday).Error; err != nil {
		return nil, err
	}

	if err := r.db.WithContext(ctx).Model(&userrepo.UserModel{}).
		Where("created_at >= ?", sevenDaysAgo).
		Count(&metrics.NewUsers7D).Error; err != nil {
		return nil, err
	}

	if err := r.db.WithContext(ctx).Raw(`
SELECT COUNT(DISTINCT user_id)
FROM exam_attempts
WHERE created_at >= ?
`, sevenDaysAgo).Scan(&metrics.ActiveUsers7D).Error; err != nil {
		log.Printf("admin dashboard: active users 7d query failed: %v", err)
	}

	if err := r.db.WithContext(ctx).Raw(`
SELECT COUNT(DISTINCT user_id)
FROM user_entitlements
WHERE entitlement_type = ?
  AND is_active = true
  AND starts_at <= ?
  AND (expires_at IS NULL OR expires_at > ?)
`, entitlementdomain.TypePremium, now, now).Scan(&metrics.PremiumUsers).Error; err != nil {
		log.Printf("admin dashboard: premium users query failed: %v", err)
	}

	if err := r.db.WithContext(ctx).Model(&userrepo.UserModel{}).
		Where("status = ?", "suspended").
		Count(&metrics.SuspendedUsers).Error; err != nil {
		log.Printf("admin dashboard: suspended users query failed: %v", err)
	}

	return metrics, nil
}
