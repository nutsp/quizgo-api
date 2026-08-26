package proofstore

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"testing"

	"github.com/google/uuid"
	"virtual-exam-api/internal/payment/domain"
)

func TestLocalStoreSavesAndOpensPrivateImage(t *testing.T) {
	root := t.TempDir()
	store, err := NewLocal(root)
	if err != nil {
		t.Fatal(err)
	}
	png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	paymentID := uuid.New()
	proof, err := store.Save(context.Background(), paymentID, "my-slip.png", bytes.NewReader(png), int64(len(png)))
	if err != nil {
		t.Fatal(err)
	}
	if proof.MIMEType != "image/png" || proof.OriginalName != "my-slip.png" || proof.StorageKey == "" {
		t.Fatalf("proof = %#v", proof)
	}
	r, err := store.Open(context.Background(), proof.StorageKey)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	got, _ := io.ReadAll(r)
	if !bytes.Equal(got, png) {
		t.Fatal("opened proof differs from uploaded data")
	}
}

func TestLocalStoreRejectsUnsupportedContent(t *testing.T) {
	store, _ := NewLocal(t.TempDir())
	_, err := store.Save(context.Background(), uuid.New(), "proof.png", bytes.NewBufferString("not an image"), 12)
	if !errors.Is(err, domain.ErrInvalidProof) {
		t.Fatalf("error = %v, want ErrInvalidProof", err)
	}
}

func TestLocalStoreRejectsFilesOverTenMegabytes(t *testing.T) {
	store, _ := NewLocal(t.TempDir())
	_, err := store.Save(context.Background(), uuid.New(), "proof.pdf", bytes.NewReader(nil), MaxProofSize+1)
	if !errors.Is(err, domain.ErrInvalidProof) {
		t.Fatalf("error = %v, want ErrInvalidProof", err)
	}
}
