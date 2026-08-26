package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	StatusAwaitingProof = "awaiting_proof"
	StatusPendingReview = "pending_review"
	StatusApproved      = "approved"
	StatusRejected      = "rejected"
)

var (
	ErrNotFound              = errors.New("payment not found")
	ErrForbidden             = errors.New("payment access forbidden")
	ErrPackageNotPurchasable = errors.New("package is not purchasable")
	ErrQRExpired             = errors.New("payment QR expired")
	ErrStateConflict         = errors.New("payment state conflict")
	ErrInvalidProof          = errors.New("invalid payment proof")
)

type PaymentRequest struct {
	ID                uuid.UUID  `json:"id"`
	UserID            uuid.UUID  `json:"userId,omitempty"`
	UserDisplayName   string     `json:"userDisplayName,omitempty"`
	UserEmail         string     `json:"userEmail,omitempty"`
	PackageID         string     `json:"packageId"`
	PackageName       string     `json:"packageName"`
	DurationMonths    int        `json:"durationMonths"`
	OriginalPrice     int        `json:"originalPrice"`
	SalePrice         int        `json:"salePrice"`
	DiscountPercent   int        `json:"discountPercent"`
	Currency          string     `json:"currency"`
	Status            string     `json:"status"`
	Provider          string     `json:"provider"`
	ProviderReference string     `json:"providerReference"`
	QRImageURL        string     `json:"qrImageUrl"`
	QRExpiresAt       time.Time  `json:"qrExpiresAt"`
	QRExpired         bool       `json:"qrExpired"`
	ProofStorageKey   *string    `json:"-"`
	ProofOriginalName *string    `json:"proofOriginalName,omitempty"`
	ProofMIMEType     *string    `json:"proofMimeType,omitempty"`
	ProofSize         int64      `json:"proofSize,omitempty"`
	ProofUploadedAt   *time.Time `json:"proofUploadedAt,omitempty"`
	ReviewedBy        *uuid.UUID `json:"reviewedBy,omitempty"`
	ReviewedAt        *time.Time `json:"reviewedAt,omitempty"`
	RejectionReason   *string    `json:"rejectionReason,omitempty"`
	EntitlementID     *uuid.UUID `json:"entitlementId,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

type QRPayment struct {
	Provider          string
	ProviderReference string
	QRImageURL        string
	ExpiresAt         time.Time
}

type Proof struct {
	StorageKey   string
	OriginalName string
	MIMEType     string
	Size         int64
}

type ListParams struct {
	Page   int
	Limit  int
	Status string
	Query  string
}

type PaginatedPayments struct {
	Items      []PaymentRequest `json:"items"`
	Page       int              `json:"page"`
	Limit      int              `json:"limit"`
	TotalItems int64            `json:"total_items"`
	TotalPages int              `json:"total_pages"`
}
