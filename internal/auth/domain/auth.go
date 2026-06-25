package domain

import "github.com/golang-jwt/jwt/v5"

type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type RegisterRequest struct {
	DisplayName string `json:"display_name" validate:"required,min=2,max=100"`
	Email       string `json:"email" validate:"required,email"`
	Password    string `json:"password" validate:"required,min=8,max=72"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type LoginResponse struct {
	AccessToken string              `json:"access_token"`
	User        UserProfileResponse `json:"user"`
}

type UserProfileResponse struct {
	ID          string  `json:"id"`
	DisplayName string  `json:"display_name"`
	Email       string  `json:"email"`
	Role        string  `json:"role"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
}
