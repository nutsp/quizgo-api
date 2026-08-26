package provider

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
	qrcode "github.com/skip2/go-qrcode"
	"virtual-exam-api/internal/payment/domain"
)

type Mock struct {
	qrImageURL string
	expiresIn  time.Duration
	now        func() time.Time
}

func NewMock(qrImageURL string, expiresIn time.Duration) *Mock {
	if expiresIn <= 0 {
		expiresIn = 30 * time.Minute
	}
	return &Mock{qrImageURL: qrImageURL, expiresIn: expiresIn, now: func() time.Time { return time.Now().UTC() }}
}

func (p *Mock) CreateQR(_ context.Context, paymentID uuid.UUID, amount int, currency string) (domain.QRPayment, error) {
	reference := "mock-" + paymentID.String()
	url := p.qrImageURL
	if url == "" {
		generated, err := mockQRDataURL(reference, amount, currency)
		if err != nil {
			return domain.QRPayment{}, err
		}
		url = generated
	}
	return domain.QRPayment{
		Provider: "mock", ProviderReference: reference, QRImageURL: url, ExpiresAt: p.now().Add(p.expiresIn),
	}, nil
}

func mockQRDataURL(reference string, amount int, currency string) (string, error) {
	payload := fmt.Sprintf("quizgo://mock-payment?reference=%s&amount=%d&currency=%s", reference, amount, currency)
	png, err := qrcode.Encode(payload, qrcode.Medium, 320)
	if err != nil {
		return "", fmt.Errorf("generate mock payment QR: %w", err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), nil
}
