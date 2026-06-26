package usecase

import (
	"context"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"virtual-exam-api/internal/apperrors"
	profiledomain "virtual-exam-api/internal/profile/domain"
	resultrepo "virtual-exam-api/internal/result/repository"
	userdomain "virtual-exam-api/internal/user/domain"
	userrepo "virtual-exam-api/internal/user/repository"
)

var displayNamePattern = regexp.MustCompile(`^[\p{L}\p{N}\s.]+$`)

type ProfileUseCase struct {
	users  userrepo.Repository
	result resultrepo.Repository
}

func NewProfileUseCase(users userrepo.Repository, result resultrepo.Repository) *ProfileUseCase {
	return &ProfileUseCase{users: users, result: result}
}

func (uc *ProfileUseCase) GetProfile(ctx context.Context, userID uuid.UUID) (*profiledomain.ProfileResponse, error) {
	user, err := uc.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, apperrors.ErrUnauthorized
	}

	resp := buildProfileResponse(user)

	stats, err := uc.result.GetOverallStats(ctx, userID)
	if err != nil {
		return nil, err
	}
	if stats != nil && stats.TotalAttempts > 0 {
		resp.Stats = &profiledomain.ProfileStats{
			TotalAttempts:       stats.TotalAttempts,
			CompletedExamSets:   stats.CompletedExamSets,
			AverageScorePercent: round1(derefFloat(stats.AverageScorePercent)),
			BestScorePercent:    round1(derefFloat(stats.BestScorePercent)),
		}
	}

	return resp, nil
}

func (uc *ProfileUseCase) UpdateProfile(ctx context.Context, userID uuid.UUID, req profiledomain.UpdateProfileRequest) (*profiledomain.UpdateProfileResponse, error) {
	displayName, err := validateDisplayName(req.DisplayName)
	if err != nil {
		return nil, err
	}

	user, err := uc.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, apperrors.ErrUnauthorized
	}

	if err := uc.users.UpdateDisplayName(ctx, userID, displayName); err != nil {
		return nil, err
	}

	user.DisplayName = displayName
	return &profiledomain.UpdateProfileResponse{
		ID:                user.ID.String(),
		Email:             user.Email,
		DisplayName:       user.DisplayName,
		PublicDisplayName: userdomain.PublicDisplayName(user.DisplayName, user.Email),
	}, nil
}

func validateDisplayName(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", apperrors.ValidationError("กรุณาระบุชื่อที่ต้องการแสดง")
	}
	if utf8.RuneCountInString(trimmed) < 2 {
		return "", apperrors.ValidationError("ชื่อต้องมีอย่างน้อย 2 ตัวอักษร")
	}
	if utf8.RuneCountInString(trimmed) > 50 {
		return "", apperrors.ValidationError("ชื่อต้องไม่เกิน 50 ตัวอักษร")
	}
	if strings.ContainsAny(trimmed, "<>\"'&/\\") {
		return "", apperrors.ValidationError("ชื่อมีอักขระที่ไม่รองรับ")
	}
	if !displayNamePattern.MatchString(trimmed) {
		return "", apperrors.ValidationError("ชื่อมีอักขระที่ไม่รองรับ")
	}
	return trimmed, nil
}

func buildProfileResponse(user *userdomain.User) *profiledomain.ProfileResponse {
	return &profiledomain.ProfileResponse{
		ID:                user.ID.String(),
		Email:             user.Email,
		DisplayName:       user.DisplayName,
		PublicDisplayName: userdomain.PublicDisplayName(user.DisplayName, user.Email),
		Role:              user.Role,
		CreatedAt:         user.CreatedAt,
	}
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

func derefFloat(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}
