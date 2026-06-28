package domain

import (
	"time"

	"github.com/google/uuid"
)

type AuditLog struct {
	ID           uuid.UUID
	ActorUserID  *uuid.UUID
	ActorEmail   string
	Action       string
	ResourceType string
	ResourceID   *uuid.UUID
	ResourceName string
	BeforeData   any
	AfterData    any
	IPAddress    string
	UserAgent    string
	Metadata     map[string]any
	CreatedAt    time.Time
}
