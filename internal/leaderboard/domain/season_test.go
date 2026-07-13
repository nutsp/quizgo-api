package domain

import (
	"testing"
	"time"
)

func TestBangkokSeasonWindowUsesLocalMonth(t *testing.T) {
	input := time.Date(2026, 6, 30, 17, 30, 0, 0, time.UTC)

	got, err := BangkokSeasonWindow(input)
	if err != nil {
		t.Fatalf("BangkokSeasonWindow() error = %v", err)
	}

	if got.Year != 2026 {
		t.Errorf("Year = %d, want 2026", got.Year)
	}
	if got.Month != 7 {
		t.Errorf("Month = %d, want 7", got.Month)
	}
	if want := time.Date(2026, 6, 30, 17, 0, 0, 0, time.UTC); !got.StartsAt.Equal(want) {
		t.Errorf("StartsAt = %s, want %s", got.StartsAt, want)
	}
	if want := time.Date(2026, 7, 31, 17, 0, 0, 0, time.UTC); !got.EndsAt.Equal(want) {
		t.Errorf("EndsAt = %s, want %s", got.EndsAt, want)
	}
}
