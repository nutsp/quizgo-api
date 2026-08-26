package config

import (
	"testing"
	"time"
)

func TestLoadPaymentConfiguration(t *testing.T) {
	t.Setenv("PAYMENT_PROOF_DIR", "/tmp/private-payment-proofs")
	t.Setenv("PAYMENT_QR_IMAGE_URL", "https://example.test/qr.png")
	t.Setenv("PAYMENT_QR_EXPIRES_IN", "45m")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PaymentProofDir != "/tmp/private-payment-proofs" || cfg.PaymentQRImageURL != "https://example.test/qr.png" || cfg.PaymentQRExpiresIn != 45*time.Minute {
		t.Fatalf("payment config = %#v", cfg)
	}
}
