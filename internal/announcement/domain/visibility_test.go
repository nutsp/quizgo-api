package domain_test

import (
	"testing"
	"time"

	"virtual-exam-api/internal/announcement/domain"
)

func TestExamScheduleVisibilityUsesBangkokCalendar(t *testing.T) {
	location := time.FixedZone("Asia/Bangkok", 7*60*60)
	examDate := time.Date(2026, 8, 15, 0, 0, 0, 0, location)
	announcement := domain.Announcement{
		Type:            domain.TypeExamSchedule,
		PublishStatus:   domain.StatusPublished,
		IsActive:        true,
		ExamDate:        &examDate,
		DaysBeforeStart: 14,
	}

	if announcement.VisibleAt(time.Date(2026, 7, 31, 12, 0, 0, 0, location)) {
		t.Fatal("announcement became visible before its derived start date")
	}
	if !announcement.VisibleAt(time.Date(2026, 8, 1, 0, 0, 0, 0, location)) {
		t.Fatal("announcement was hidden on its derived start date")
	}
	daysLeft := announcement.DaysLeft(time.Date(2026, 8, 1, 12, 0, 0, 0, location))
	if daysLeft == nil || *daysLeft != 14 {
		t.Fatalf("days left = %v, want 14", daysLeft)
	}
}

func TestPinnedScheduleRemainsVisibleAfterExamDate(t *testing.T) {
	location := time.FixedZone("Asia/Bangkok", 7*60*60)
	examDate := time.Date(2026, 8, 15, 0, 0, 0, 0, location)
	now := time.Date(2026, 8, 16, 0, 0, 0, 0, location)
	announcement := domain.Announcement{
		Type: domain.TypeExamSchedule, PublishStatus: domain.StatusPublished,
		IsActive: true, IsPinned: true, ExamDate: &examDate,
	}

	if !announcement.VisibleAt(now) {
		t.Fatal("pinned schedule should remain visible after exam date")
	}
	announcement.IsPinned = false
	if announcement.VisibleAt(now) {
		t.Fatal("unpinned schedule should be hidden after exam date")
	}
}
