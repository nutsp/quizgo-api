package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

type User struct {
	ID           uuid.UUID
	DisplayName  string
	Email        string
	PasswordHash string
	Role         string
	AvatarURL    *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type UserProfile struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
	Role        string    `json:"role"`
	AvatarURL   *string   `json:"avatar_url,omitempty"`
}

func (u *User) ToProfile() UserProfile {
	return UserProfile{
		ID:          u.ID,
		DisplayName: u.DisplayName,
		Email:       u.Email,
		Role:        u.Role,
		AvatarURL:   u.AvatarURL,
	}
}
