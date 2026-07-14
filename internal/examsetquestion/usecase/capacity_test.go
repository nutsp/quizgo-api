package usecase

import (
	"strings"
	"testing"
)

func TestValidateRuleCapacityAllowsMaximum(t *testing.T) {
	if err := validateRuleCapacity(0, 12, 12); err != nil {
		t.Fatalf("expected capacity equality to pass, got %v", err)
	}
}

func TestValidateRuleCapacityRejectsAboveMaximum(t *testing.T) {
	err := validateRuleCapacity(1, 13, 12)
	if err == nil {
		t.Fatal("expected capacity validation error")
	}
	if !strings.Contains(err.Error(), "ส่วนที่ 2 ของโครงสร้าง") || !strings.Contains(err.Error(), "12") {
		t.Fatalf("unexpected error: %v", err)
	}
}
