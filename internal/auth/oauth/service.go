package oauth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	authdomain "virtual-exam-api/internal/auth/domain"
	"virtual-exam-api/internal/apperrors"
	oauthdomain "virtual-exam-api/internal/auth/oauth/domain"
	oauthrepo "virtual-exam-api/internal/auth/oauth/repository"
	"virtual-exam-api/internal/config"
	userdomain "virtual-exam-api/internal/user/domain"
	userrepo "virtual-exam-api/internal/user/repository"
)

type TokenIssuer interface {
	IssueTokenForUser(user *userdomain.User) (*authdomain.LoginResponse, error)
}

type Service struct {
	users       userrepo.Repository
	oauth       oauthrepo.Repository
	google      *GoogleClient
	state       *StateManager
	tokenIssuer TokenIssuer
	frontendURL string
}

func NewService(
	users userrepo.Repository,
	oauth oauthrepo.Repository,
	tokenIssuer TokenIssuer,
	cfg *config.Config,
) *Service {
	return &Service{
		users:       users,
		oauth:       oauth,
		google:      NewGoogleClient(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURL),
		state:       NewStateManager(cfg.OAuthStateSecret),
		tokenIssuer: tokenIssuer,
		frontendURL: cfg.FrontendURL,
	}
}

func (s *Service) BeginGoogleLogin(w http.ResponseWriter, redirect string) (string, error) {
	if s.google.clientID == "" || s.google.clientSecret == "" {
		return "", errors.New("google oauth not configured")
	}
	state, err := s.state.Create(w, redirect)
	if err != nil {
		return "", err
	}
	return s.google.AuthURL(state), nil
}

func (s *Service) HandleGoogleCallback(w http.ResponseWriter, r *http.Request, code, state string) (string, *authdomain.LoginResponse, error) {
	redirect, err := s.state.Validate(w, r, state)
	if err != nil {
		return BuildFrontendErrorURL(s.frontendURL), nil, err
	}
	if code == "" {
		return BuildFrontendErrorURL(s.frontendURL), nil, errors.New("missing code")
	}

	loginResp, err := s.authenticateGoogle(r.Context(), code)
	if err != nil {
		return BuildFrontendErrorURL(s.frontendURL), nil, err
	}

	return BuildFrontendCallbackURL(s.frontendURL, loginResp.AccessToken, redirect), loginResp, nil
}

func (s *Service) authenticateGoogle(ctx context.Context, code string) (*authdomain.LoginResponse, error) {
	info, err := s.google.ExchangeCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if !info.EmailVerified {
		return nil, errors.New("google email not verified")
	}
	if info.Sub == "" || info.Email == "" {
		return nil, errors.New("google profile incomplete")
	}

	email := strings.ToLower(info.Email)
	providerEmail := strPtr(email)
	providerName := strPtr(info.Name)
	providerAvatar := strPtr(info.Picture)

	account, err := s.oauth.FindByProviderAndUserID(ctx, oauthdomain.ProviderGoogle, info.Sub)
	if err != nil {
		return nil, err
	}

	if account != nil {
		account.ProviderEmail = providerEmail
		account.ProviderName = providerName
		account.ProviderAvatarURL = providerAvatar
		if err := s.oauth.UpdateProviderProfile(ctx, account); err != nil {
			return nil, err
		}

		user, err := s.users.FindByID(ctx, account.UserID)
		if err != nil {
			return nil, err
		}
		if user == nil {
			return nil, errors.New("linked user not found")
		}
		if !user.CanLogin() {
			return nil, apperrors.ErrAccountSuspended
		}
		if err := s.users.UpdateLastLoginAt(ctx, user.ID, time.Now().UTC()); err != nil {
			return nil, err
		}
		return s.tokenIssuer.IssueTokenForUser(user)
	}

	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	if user != nil {
		if strings.TrimSpace(user.DisplayName) == "" && info.Name != "" {
			if err := s.users.UpdateDisplayNameIfEmpty(ctx, user.ID, info.Name); err != nil {
				return nil, err
			}
			user.DisplayName = info.Name
		}

		newAccount := &oauthdomain.OAuthAccount{
			ID:                uuid.New(),
			UserID:            user.ID,
			Provider:          oauthdomain.ProviderGoogle,
			ProviderUserID:    info.Sub,
			ProviderEmail:     providerEmail,
			ProviderName:      providerName,
			ProviderAvatarURL: providerAvatar,
		}
		if err := s.oauth.Create(ctx, newAccount); err != nil {
			return nil, err
		}
		if !user.CanLogin() {
			return nil, apperrors.ErrAccountSuspended
		}
		if err := s.users.UpdateLastLoginAt(ctx, user.ID, time.Now().UTC()); err != nil {
			return nil, err
		}
		return s.tokenIssuer.IssueTokenForUser(user)
	}

	displayName := info.Name
	if strings.TrimSpace(displayName) == "" {
		displayName = strings.Split(email, "@")[0]
	}

	newUser := &userdomain.User{
		ID:          uuid.New(),
		DisplayName: displayName,
		Email:       email,
		Role:        userdomain.RoleUser,
		Status:      userdomain.StatusActive,
	}

	if err := s.users.CreateOAuthUser(ctx, newUser); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			existing, findErr := s.users.FindByEmail(ctx, email)
			if findErr != nil || existing == nil {
				return nil, err
			}
			user = existing
		} else {
			return nil, err
		}
	} else {
		user = newUser
	}

	newAccount := &oauthdomain.OAuthAccount{
		ID:                uuid.New(),
		UserID:            user.ID,
		Provider:          oauthdomain.ProviderGoogle,
		ProviderUserID:    info.Sub,
		ProviderEmail:     providerEmail,
		ProviderName:      providerName,
		ProviderAvatarURL: providerAvatar,
	}
	if err := s.oauth.Create(ctx, newAccount); err != nil {
		return nil, err
	}

	if !user.CanLogin() {
		return nil, apperrors.ErrAccountSuspended
	}
	if err := s.users.UpdateLastLoginAt(ctx, user.ID, time.Now().UTC()); err != nil {
		return nil, err
	}
	return s.tokenIssuer.IssueTokenForUser(user)
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (s *Service) FrontendURL() string {
	return s.frontendURL
}
