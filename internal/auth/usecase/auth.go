package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	authdomain "virtual-exam-api/internal/auth/domain"
	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/config"
	userdomain "virtual-exam-api/internal/user/domain"
	userrepo "virtual-exam-api/internal/user/repository"
)

type AuthUseCase struct {
	users     userrepo.Repository
	validator *validator.Validate
	jwtSecret []byte
	jwtExpiry time.Duration
}

func NewAuthUseCase(users userrepo.Repository, cfg *config.Config) *AuthUseCase {
	return &AuthUseCase{
		users:     users,
		validator: validator.New(),
		jwtSecret: []byte(cfg.JWTSecret),
		jwtExpiry: cfg.JWTExpiresIn,
	}
}

func (uc *AuthUseCase) Register(ctx context.Context, req authdomain.RegisterRequest) (*authdomain.LoginResponse, error) {
	if err := uc.validator.Struct(req); err != nil {
		return nil, apperrors.ErrInvalidInput
	}

	existing, err := uc.users.FindByEmail(ctx, strings.ToLower(req.Email))
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, apperrors.ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &userdomain.User{
		ID:           uuid.New(),
		DisplayName:  req.DisplayName,
		Email:        strings.ToLower(req.Email),
		PasswordHash: string(hash),
		Role:         userdomain.RoleUser,
		Status:       userdomain.StatusActive,
	}

	if err := uc.users.Create(ctx, user); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, apperrors.ErrEmailTaken
		}
		return nil, err
	}

	return uc.buildLoginResponse(user)
}

func (uc *AuthUseCase) Login(ctx context.Context, req authdomain.LoginRequest) (*authdomain.LoginResponse, error) {
	if err := uc.validator.Struct(req); err != nil {
		return nil, apperrors.ErrInvalidInput
	}

	user, err := uc.users.FindByEmail(ctx, strings.ToLower(req.Email))
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, apperrors.ErrInvalidCredentials
	}

	if !user.CanLogin() {
		return nil, apperrors.ErrAccountSuspended
	}

	if user.PasswordHash == "" {
		return nil, apperrors.ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, apperrors.ErrInvalidCredentials
	}

	if err := uc.users.UpdateLastLoginAt(ctx, user.ID, time.Now().UTC()); err != nil {
		return nil, err
	}

	return uc.buildLoginResponse(user)
}

func (uc *AuthUseCase) Me(ctx context.Context, userID uuid.UUID) (*authdomain.UserProfileResponse, error) {
	user, err := uc.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, apperrors.ErrUnauthorized
	}

	profile := user.ToProfile()
	publicName := userdomain.PublicDisplayName(user.DisplayName, user.Email)
	return &authdomain.UserProfileResponse{
		ID:                profile.ID.String(),
		DisplayName:       profile.DisplayName,
		PublicDisplayName: publicName,
		Email:             profile.Email,
		Role:              profile.Role,
		AvatarURL:         profile.AvatarURL,
	}, nil
}

func (uc *AuthUseCase) ParseToken(tokenStr string) (*authdomain.Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &authdomain.Claims{}, func(token *jwt.Token) (any, error) {
		return uc.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*authdomain.Claims)
	if !ok || !token.Valid {
		return nil, apperrors.ErrUnauthorized
	}
	return claims, nil
}

func (uc *AuthUseCase) IssueTokenForUser(user *userdomain.User) (*authdomain.LoginResponse, error) {
	return uc.buildLoginResponse(user)
}

func (uc *AuthUseCase) buildLoginResponse(user *userdomain.User) (*authdomain.LoginResponse, error) {
	now := time.Now().UTC()
	claims := &authdomain.Claims{
		UserID: user.ID.String(),
		Email:  user.Email,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(uc.jwtExpiry)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString(uc.jwtSecret)
	if err != nil {
		return nil, err
	}

	profile := user.ToProfile()
	publicName := userdomain.PublicDisplayName(user.DisplayName, user.Email)
	return &authdomain.LoginResponse{
		AccessToken: tokenStr,
		User: authdomain.UserProfileResponse{
			ID:                profile.ID.String(),
			DisplayName:       profile.DisplayName,
			PublicDisplayName: publicName,
			Email:             profile.Email,
			Role:              profile.Role,
			AvatarURL:         profile.AvatarURL,
		},
	}, nil
}
