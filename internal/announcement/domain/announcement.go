package domain

import (
	"strconv"
	"time"

	"github.com/google/uuid"
)

type Type string

const (
	TypeGeneral      Type = "general"
	TypeExamSchedule Type = "exam_schedule"
	TypeExamUpdate   Type = "exam_update"
	TypePromotion    Type = "promotion"
	TypeMaintenance  Type = "maintenance"
	TypeSystem       Type = "system"
)

type PublishStatus string

const (
	StatusDraft     PublishStatus = "draft"
	StatusPublished PublishStatus = "published"
	StatusArchived  PublishStatus = "archived"
)

var bangkokLocation = time.FixedZone("Asia/Bangkok", 7*60*60)

type ExamTrackSummary struct {
	ID   uuid.UUID
	Code string
	Name string
}

type ExamSetSummary struct {
	ID        uuid.UUID
	Code      string
	Title     string
	SortOrder int
}

type Announcement struct {
	ID              uuid.UUID
	Title           string
	Slug            string
	Summary         string
	Content         string
	Type            Type
	Priority        int
	IsPinned        bool
	IsActive        bool
	PublishStatus   PublishStatus
	StartsAt        *time.Time
	EndsAt          *time.Time
	ExamTrackID     *uuid.UUID
	ExamDate        *time.Time
	DaysBeforeStart int
	CTALabel        string
	CTAURL          string
	CreatedBy       *uuid.UUID
	UpdatedBy       *uuid.UUID
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ExamTrack       *ExamTrackSummary
	RecommendedSets []ExamSetSummary
}

func (t Type) Valid() bool {
	switch t {
	case TypeGeneral, TypeExamSchedule, TypeExamUpdate, TypePromotion, TypeMaintenance, TypeSystem:
		return true
	default:
		return false
	}
}

func (s PublishStatus) Valid() bool {
	switch s {
	case StatusDraft, StatusPublished, StatusArchived:
		return true
	default:
		return false
	}
}

func (a Announcement) VisibleAt(now time.Time) bool {
	if a.PublishStatus != StatusPublished || !a.IsActive {
		return false
	}
	if a.StartsAt != nil && now.Before(*a.StartsAt) {
		return false
	}
	if a.EndsAt != nil && now.After(*a.EndsAt) {
		return false
	}
	if a.Type != TypeExamSchedule || a.ExamDate == nil {
		return true
	}

	if a.StartsAt == nil {
		effectiveStart := bangkokDate(*a.ExamDate).AddDate(0, 0, -a.DaysBeforeStart)
		if bangkokDate(now).Before(effectiveStart) {
			return false
		}
	}

	return a.IsPinned || !bangkokDate(now).After(bangkokDate(*a.ExamDate))
}

func (a Announcement) DaysLeft(now time.Time) *int {
	if a.Type != TypeExamSchedule || a.ExamDate == nil {
		return nil
	}
	days := int(bangkokDate(*a.ExamDate).Sub(bangkokDate(now)).Hours() / 24)
	return &days
}

func (a Announcement) ScheduleText(now time.Time) string {
	days := a.DaysLeft(now)
	if days == nil || *days < 0 {
		return ""
	}
	switch *days {
	case 0:
		return "สอบวันนี้"
	case 1:
		return "เหลืออีก 1 วัน"
	default:
		return "เหลืออีก " + strconv.Itoa(*days) + " วัน"
	}
}

func bangkokDate(value time.Time) time.Time {
	local := value.In(bangkokLocation)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, bangkokLocation)
}
