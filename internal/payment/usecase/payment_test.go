package usecase

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"virtual-exam-api/internal/payment/catalog"
	"virtual-exam-api/internal/payment/domain"
)

type fakeRepository struct {
	items        map[uuid.UUID]*domain.PaymentRequest
	createCalls  int
	approveCalls int
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{items: map[uuid.UUID]*domain.PaymentRequest{}}
}

func (f *fakeRepository) Create(_ context.Context, payment *domain.PaymentRequest) error {
	f.createCalls++
	copy := *payment
	f.items[payment.ID] = &copy
	return nil
}

func (f *fakeRepository) FindReusable(_ context.Context, userID uuid.UUID, packageID string, now time.Time) (*domain.PaymentRequest, error) {
	for _, item := range f.items {
		if item.UserID == userID && item.PackageID == packageID && item.Status == domain.StatusAwaitingProof && item.QRExpiresAt.After(now) {
			copy := *item
			return &copy, nil
		}
	}
	return nil, nil
}

func (f *fakeRepository) FindByID(_ context.Context, id uuid.UUID) (*domain.PaymentRequest, error) {
	item := f.items[id]
	if item == nil {
		return nil, nil
	}
	copy := *item
	return &copy, nil
}

func (f *fakeRepository) AttachProof(_ context.Context, id uuid.UUID, proof domain.Proof) error {
	item := f.items[id]
	item.ProofStorageKey = &proof.StorageKey
	item.ProofOriginalName = &proof.OriginalName
	item.ProofMIMEType = &proof.MIMEType
	item.ProofSize = proof.Size
	item.Status = domain.StatusPendingReview
	return nil
}

func (f *fakeRepository) List(_ context.Context, _ domain.ListParams) ([]domain.PaymentRequest, int64, error) {
	return nil, 0, nil
}

func (f *fakeRepository) Approve(_ context.Context, id, reviewerID uuid.UUID, now time.Time) (*domain.PaymentRequest, error) {
	f.approveCalls++
	item := f.items[id]
	if item.Status != domain.StatusPendingReview {
		return nil, domain.ErrStateConflict
	}
	item.Status = domain.StatusApproved
	item.ReviewedBy = &reviewerID
	item.ReviewedAt = &now
	copy := *item
	return &copy, nil
}

func (f *fakeRepository) Reject(_ context.Context, id, reviewerID uuid.UUID, reason string, now time.Time) (*domain.PaymentRequest, error) {
	item := f.items[id]
	if item.Status != domain.StatusPendingReview {
		return nil, domain.ErrStateConflict
	}
	item.Status = domain.StatusRejected
	item.ReviewedBy = &reviewerID
	item.ReviewedAt = &now
	item.RejectionReason = &reason
	copy := *item
	return &copy, nil
}

type fakeProvider struct{ calls int }

func (f *fakeProvider) CreateQR(_ context.Context, paymentID uuid.UUID, amount int, currency string) (domain.QRPayment, error) {
	f.calls++
	return domain.QRPayment{
		Provider:          "mock",
		ProviderReference: "mock-" + paymentID.String(),
		QRImageURL:        "https://example.test/qr.png",
		ExpiresAt:         time.Date(2026, 7, 17, 13, 30, 0, 0, time.UTC),
	}, nil
}

type fakeProofStore struct{ saved bool }

func (f *fakeProofStore) Save(_ context.Context, paymentID uuid.UUID, originalName string, src io.Reader, size int64) (domain.Proof, error) {
	f.saved = true
	return domain.Proof{StorageKey: paymentID.String() + "/proof.png", OriginalName: originalName, MIMEType: "image/png", Size: size}, nil
}

func (f *fakeProofStore) Open(_ context.Context, _ string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(nil)), nil
}

func testUseCase(t *testing.T) (*UseCase, *fakeRepository, *fakeProvider, *fakeProofStore, time.Time) {
	t.Helper()
	packages, err := catalog.LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 17, 13, 0, 0, 0, time.UTC)
	repo := newFakeRepository()
	provider := &fakeProvider{}
	store := &fakeProofStore{}
	uc := New(repo, provider, store, packages)
	uc.now = func() time.Time { return now }
	return uc, repo, provider, store, now
}

func TestCreatePaymentUsesCatalogAndReusesValidRequest(t *testing.T) {
	uc, repo, provider, _, _ := testUseCase(t)
	userID := uuid.New()

	first, err := uc.CreatePayment(context.Background(), userID, "premium-3m")
	if err != nil {
		t.Fatal(err)
	}
	second, err := uc.CreatePayment(context.Background(), userID, "premium-3m")
	if err != nil {
		t.Fatal(err)
	}

	if first.ID != second.ID || repo.createCalls != 1 || provider.calls != 1 {
		t.Fatalf("payment was not reused: ids %s/%s create=%d provider=%d", first.ID, second.ID, repo.createCalls, provider.calls)
	}
	if first.SalePrice != 399 || first.DurationMonths != 3 || first.DiscountPercent != 20 {
		t.Fatalf("snapshot = %#v", first)
	}
}

func TestCreatePaymentRejectsNonPurchasablePackage(t *testing.T) {
	uc, _, _, _, _ := testUseCase(t)
	_, err := uc.CreatePayment(context.Background(), uuid.New(), "business")
	if !errors.Is(err, domain.ErrPackageNotPurchasable) {
		t.Fatalf("error = %v, want ErrPackageNotPurchasable", err)
	}
}

func TestUploadProofRequiresOwnerAndValidQR(t *testing.T) {
	uc, repo, _, store, now := testUseCase(t)
	owner := uuid.New()
	payment, err := uc.CreatePayment(context.Background(), owner, "premium-1m")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := uc.UploadProof(context.Background(), uuid.New(), payment.ID, "proof.png", bytes.NewReader([]byte("png")), 3); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("wrong owner error = %v", err)
	}

	repo.items[payment.ID].QRExpiresAt = now.Add(-time.Minute)
	if _, err := uc.UploadProof(context.Background(), owner, payment.ID, "proof.png", bytes.NewReader([]byte("png")), 3); !errors.Is(err, domain.ErrQRExpired) {
		t.Fatalf("expired QR error = %v", err)
	}

	repo.items[payment.ID].QRExpiresAt = now.Add(time.Minute)
	updated, err := uc.UploadProof(context.Background(), owner, payment.ID, "proof.png", bytes.NewReader([]byte("png")), 3)
	if err != nil {
		t.Fatal(err)
	}
	if !store.saved || updated.Status != domain.StatusPendingReview {
		t.Fatalf("saved/status = %v/%s", store.saved, updated.Status)
	}
}

func TestApproveIsIdempotentAtUseCaseBoundary(t *testing.T) {
	uc, repo, _, _, _ := testUseCase(t)
	owner := uuid.New()
	reviewer := uuid.New()
	payment, _ := uc.CreatePayment(context.Background(), owner, "premium-12m")
	repo.items[payment.ID].Status = domain.StatusPendingReview

	approved, err := uc.Approve(context.Background(), payment.ID, reviewer)
	if err != nil || approved.Status != domain.StatusApproved {
		t.Fatalf("Approve() = %#v, %v", approved, err)
	}
	if _, err := uc.Approve(context.Background(), payment.ID, reviewer); !errors.Is(err, domain.ErrStateConflict) {
		t.Fatalf("second approval error = %v", err)
	}
	if repo.approveCalls != 2 {
		t.Fatalf("approve calls = %d", repo.approveCalls)
	}
}
