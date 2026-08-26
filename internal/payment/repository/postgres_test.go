package repository

import (
	"testing"
	"time"
)

func TestNextEntitlementWindowStartsAfterExistingPremium(t *testing.T) {
	now := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
	existing := now.AddDate(0, 2, 0)
	starts, expires := nextEntitlementWindow(now, &existing, 3)
	if !starts.Equal(existing) {
		t.Fatalf("starts = %v, want %v", starts, existing)
	}
	if !expires.Equal(existing.AddDate(0, 3, 0)) {
		t.Fatalf("expires = %v", expires)
	}
}

func TestNextEntitlementWindowStartsNowWithoutExistingPremium(t *testing.T) {
	now := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
	starts, expires := nextEntitlementWindow(now, nil, 1)
	if !starts.Equal(now) || !expires.Equal(now.AddDate(0, 1, 0)) {
		t.Fatalf("window = %v..%v", starts, expires)
	}
}
