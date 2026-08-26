package usecase

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
	"virtual-exam-api/internal/payment/catalog"
	"virtual-exam-api/internal/payment/domain"
)

type Repository interface {
	Create(context.Context, *domain.PaymentRequest) error
	FindReusable(context.Context, uuid.UUID, string, time.Time) (*domain.PaymentRequest, error)
	FindByID(context.Context, uuid.UUID) (*domain.PaymentRequest, error)
	AttachProof(context.Context, uuid.UUID, domain.Proof) error
	List(context.Context, domain.ListParams) ([]domain.PaymentRequest, int64, error)
	Approve(context.Context, uuid.UUID, uuid.UUID, time.Time) (*domain.PaymentRequest, error)
	Reject(context.Context, uuid.UUID, uuid.UUID, string, time.Time) (*domain.PaymentRequest, error)
}

type Provider interface {
	CreateQR(context.Context, uuid.UUID, int, string) (domain.QRPayment, error)
}

type ProofStore interface {
	Save(context.Context, uuid.UUID, string, io.Reader, int64) (domain.Proof, error)
	Open(context.Context, string) (io.ReadCloser, error)
}

type UseCase struct {
	repo            Repository
	provider        Provider
	proofs          ProofStore
	packages        []catalog.Package
	now             func() time.Time
	onAccessChanged func(context.Context, uuid.UUID)
}

func (uc *UseCase) SetOnAccessChanged(fn func(context.Context, uuid.UUID)) {
	uc.onAccessChanged = fn
}

func New(repo Repository, provider Provider, proofs ProofStore, packages []catalog.Package) *UseCase {
	return &UseCase{repo: repo, provider: provider, proofs: proofs, packages: packages, now: func() time.Time { return time.Now().UTC() }}
}

func (uc *UseCase) Packages() []catalog.Package {
	return append([]catalog.Package(nil), uc.packages...)
}

func (uc *UseCase) CreatePayment(ctx context.Context, userID uuid.UUID, packageID string) (*domain.PaymentRequest, error) {
	pkg, ok := catalog.Find(uc.packages, packageID)
	if !ok || !pkg.Purchasable {
		return nil, domain.ErrPackageNotPurchasable
	}
	now := uc.now()
	existing, err := uc.repo.FindReusable(ctx, userID, packageID, now)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		existing.QRExpired = !existing.QRExpiresAt.After(now)
		return existing, nil
	}

	payment := &domain.PaymentRequest{
		ID:              uuid.New(),
		UserID:          userID,
		PackageID:       pkg.ID,
		PackageName:     pkg.Name,
		DurationMonths:  pkg.DurationMonths,
		OriginalPrice:   pkg.OriginalPrice,
		SalePrice:       pkg.SalePrice,
		DiscountPercent: pkg.DiscountPercent,
		Currency:        pkg.Currency,
		Status:          domain.StatusAwaitingProof,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	qr, err := uc.provider.CreateQR(ctx, payment.ID, payment.SalePrice, payment.Currency)
	if err != nil {
		return nil, err
	}
	payment.Provider = qr.Provider
	payment.ProviderReference = qr.ProviderReference
	payment.QRImageURL = qr.QRImageURL
	payment.QRExpiresAt = qr.ExpiresAt
	if err := uc.repo.Create(ctx, payment); err != nil {
		return nil, err
	}
	return payment, nil
}

func (uc *UseCase) GetPayment(ctx context.Context, requesterID, id uuid.UUID, isAdmin bool) (*domain.PaymentRequest, error) {
	payment, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if payment == nil {
		return nil, domain.ErrNotFound
	}
	if !isAdmin && payment.UserID != requesterID {
		return nil, domain.ErrForbidden
	}
	payment.QRExpired = !payment.QRExpiresAt.After(uc.now())
	return payment, nil
}

func (uc *UseCase) UploadProof(ctx context.Context, userID, id uuid.UUID, filename string, src io.Reader, size int64) (*domain.PaymentRequest, error) {
	payment, err := uc.GetPayment(ctx, userID, id, false)
	if err != nil {
		return nil, err
	}
	if payment.Status != domain.StatusAwaitingProof {
		return nil, domain.ErrStateConflict
	}
	if payment.QRExpired {
		return nil, domain.ErrQRExpired
	}
	proof, err := uc.proofs.Save(ctx, id, filename, src, size)
	if err != nil {
		return nil, err
	}
	if err := uc.repo.AttachProof(ctx, id, proof); err != nil {
		return nil, err
	}
	return uc.GetPayment(ctx, userID, id, false)
}

func (uc *UseCase) OpenProof(ctx context.Context, requesterID, id uuid.UUID, isAdmin bool) (io.ReadCloser, string, string, error) {
	payment, err := uc.GetPayment(ctx, requesterID, id, isAdmin)
	if err != nil {
		return nil, "", "", err
	}
	if payment.ProofStorageKey == nil || payment.ProofOriginalName == nil || payment.ProofMIMEType == nil {
		return nil, "", "", domain.ErrNotFound
	}
	r, err := uc.proofs.Open(ctx, *payment.ProofStorageKey)
	if err != nil {
		return nil, "", "", err
	}
	return r, *payment.ProofOriginalName, *payment.ProofMIMEType, nil
}

func (uc *UseCase) List(ctx context.Context, params domain.ListParams) (*domain.PaginatedPayments, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.Limit < 1 || params.Limit > 100 {
		params.Limit = 20
	}
	items, total, err := uc.repo.List(ctx, params)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].QRExpired = !items[i].QRExpiresAt.After(uc.now())
	}
	pages := 0
	if total > 0 {
		pages = int((total + int64(params.Limit) - 1) / int64(params.Limit))
	}
	return &domain.PaginatedPayments{Items: items, Page: params.Page, Limit: params.Limit, TotalItems: total, TotalPages: pages}, nil
}

func (uc *UseCase) Approve(ctx context.Context, id, reviewerID uuid.UUID) (*domain.PaymentRequest, error) {
	payment, err := uc.repo.Approve(ctx, id, reviewerID, uc.now())
	if err == nil && payment != nil && uc.onAccessChanged != nil {
		uc.onAccessChanged(ctx, payment.UserID)
	}
	return payment, err
}

func (uc *UseCase) Reject(ctx context.Context, id, reviewerID uuid.UUID, reason string) (*domain.PaymentRequest, error) {
	if reason == "" {
		return nil, domain.ErrStateConflict
	}
	return uc.repo.Reject(ctx, id, reviewerID, reason, uc.now())
}
