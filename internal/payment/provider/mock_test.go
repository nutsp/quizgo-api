package provider

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMockProviderReturnsAPIQRContract(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	provider := NewMock("", 30*time.Minute)
	provider.now = func() time.Time { return now }
	paymentID := uuid.New()

	qr, err := provider.CreateQR(context.Background(), paymentID, 399, "THB")
	if err != nil {
		t.Fatal(err)
	}
	if qr.Provider != "mock" || !strings.Contains(qr.ProviderReference, paymentID.String()) {
		t.Fatalf("provider response = %#v", qr)
	}
	if !strings.HasPrefix(qr.QRImageURL, "data:image/png;base64,") {
		t.Fatalf("qr image = %q", qr.QRImageURL)
	}
	if !qr.ExpiresAt.Equal(now.Add(30 * time.Minute)) {
		t.Fatalf("expiry = %v", qr.ExpiresAt)
	}
}

func TestMockProviderUsesConfiguredQRImageURL(t *testing.T) {
	provider := NewMock("https://cdn.example.test/payment-qr.png", time.Hour)
	qr, err := provider.CreateQR(context.Background(), uuid.New(), 149, "THB")
	if err != nil {
		t.Fatal(err)
	}
	if qr.QRImageURL != "https://cdn.example.test/payment-qr.png" {
		t.Fatalf("qr image = %q", qr.QRImageURL)
	}
}
