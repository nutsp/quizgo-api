package domain

import (
	"time"

	"github.com/google/uuid"
)

const ProviderGoogle = "google"

type OAuthAccount struct {
	ID                uuid.UUID
	UserID            uuid.UUID
	Provider          string
	ProviderUserID    string
	ProviderEmail     *string
	ProviderName      *string
	ProviderAvatarURL *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type GoogleUserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}
