package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	EventLoginSuccess                = "login_success"
	EventLoginFailed                 = "login_failed"
	EventLogout                      = "logout"
	EventOAuthLoginSuccess           = "oauth_login_success"
	EventOAuthLoginFailed            = "oauth_login_failed"
	EventAccountSuspendedLoginBlocked = "account_suspended_login_blocked"
)

type AccessLog struct {
	ID        uuid.UUID
	UserID    *uuid.UUID
	Email     string
	EventType string
	Success   bool
	IPAddress string
	UserAgent string
	Message   string
	Metadata  map[string]any
	CreatedAt time.Time
}
