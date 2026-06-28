package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	TypeExamSet = "exam_set"
	TypePremium = "premium"

	RefTypeExamSet = "exam_set"

	SourceManual     = "manual"
	SourcePurchase   = "purchase"
	SourceSubscription = "subscription"

	StatusActive  = "active"
	StatusExpired = "expired"
	StatusRevoked = "revoked"
	StatusPending = "pending"

	ReasonLoginRequired              = "LOGIN_REQUIRED"
	ReasonAccessRequired             = "ACCESS_REQUIRED"
	ReasonPremiumRequired            = "PREMIUM_REQUIRED"
	ReasonAccessRequiredOrPremium    = "ACCESS_REQUIRED_OR_PREMIUM"
	ReasonPrivateExamAccessRequired  = "PRIVATE_EXAM_ACCESS_REQUIRED"
	ReasonExamNotAvailable           = "EXAM_NOT_AVAILABLE"

	AccessSourceFree           = "free"
	AccessSourceSinglePurchase = "single_purchase"
	AccessSourcePremium        = "premium"
	AccessSourcePrivateGrant   = "private_grant"
	AccessSourceManualGrant    = "manual_grant"
	AccessSourceAdminGrant     = "admin_grant"
)

type Entitlement struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	EntitlementType string
	RefType         *string
	RefID           *uuid.UUID
	RefName         *string
	Source          string
	StartsAt        time.Time
	ExpiresAt       *time.Time
	IsActive        bool
	Notes           *string
	GrantedBy       *uuid.UUID
	GrantedByName   *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (e Entitlement) Status(now time.Time) string {
	if !e.IsActive {
		return StatusRevoked
	}
	if e.StartsAt.After(now) {
		return StatusPending
	}
	if e.ExpiresAt != nil && !e.ExpiresAt.After(now) {
		return StatusExpired
	}
	return StatusActive
}

func (e Entitlement) IsCurrentlyActive(now time.Time) bool {
	return e.Status(now) == StatusActive
}

type ExamSetAccessResult struct {
	CanStart            bool       `json:"can_start"`
	Reason              *string    `json:"reason,omitempty"`
	HasExamSetAccess    bool       `json:"has_exam_set_access"`
	HasPremium          bool       `json:"has_premium"`
	AvailableOptions    []string   `json:"available_options,omitempty"`
	AccessType          string     `json:"access_type"`
	AccessSource        string     `json:"access_source,omitempty"`
	EntitlementID       *uuid.UUID `json:"entitlement_id,omitempty"`
	AccessExpiresAt     *time.Time `json:"access_expires_at,omitempty"`
	AllowSinglePurchase bool       `json:"allow_single_purchase"`
	PriceAmount         int        `json:"price_amount"`
	PriceCurrency       string     `json:"price_currency"`
	UnlockURL           string     `json:"-"`
}

// AccessResult is kept as an alias for internal backward compatibility.
type AccessResult = ExamSetAccessResult

type GrantExamSetAccessInput struct {
	UserID     uuid.UUID
	ExamSetID  uuid.UUID
	ExpiresAt  *time.Time
	Notes      *string
	GrantedBy  uuid.UUID
	Source     string
}

type GrantPremiumAccessInput struct {
	UserID    uuid.UUID
	ExpiresAt time.Time
	Notes     *string
	GrantedBy uuid.UUID
	Source    string
}

type ListFilter struct {
	UserID uuid.UUID
	Page   int
	Limit  int
}

type PaginatedEntitlements struct {
	Items      []Entitlement `json:"items"`
	Page       int           `json:"page"`
	Limit      int           `json:"limit"`
	TotalItems int64         `json:"total_items"`
	TotalPages int           `json:"total_pages"`
}

const (
	AccessTypeFree    = "free"
	AccessTypeExamSet = "exam_set"
	AccessTypePremium = "premium"
)

type AccessSummary struct {
	DisplayAccessType  string
	HasPremium         bool
	ActiveExamSetCount int
	PremiumExpiresAt   *time.Time
}

func BuildAccessSummary(hasPremium bool, examSetCount int, premiumExpiresAt *time.Time) AccessSummary {
	displayAccessType := AccessTypeFree
	if hasPremium {
		displayAccessType = AccessTypePremium
	} else if examSetCount > 0 {
		displayAccessType = AccessTypeExamSet
	}
	return AccessSummary{
		DisplayAccessType:  displayAccessType,
		HasPremium:         hasPremium,
		ActiveExamSetCount: examSetCount,
		PremiumExpiresAt:   premiumExpiresAt,
	}
}
