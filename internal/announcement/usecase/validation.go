package usecase

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"

	"virtual-exam-api/internal/announcement/domain"
	"virtual-exam-api/internal/apperrors"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func ValidateInput(input MutationInput) error {
	if strings.TrimSpace(input.Title) == "" {
		return apperrors.ValidationError("กรุณากรอกหัวข้อประกาศ")
	}
	if !slugPattern.MatchString(input.Slug) {
		return apperrors.ValidationError("Slug ต้องเป็นตัวพิมพ์เล็ก ตัวเลข หรือขีดกลาง")
	}
	if !input.Type.Valid() {
		return apperrors.ValidationError("ประเภทประกาศไม่ถูกต้อง")
	}
	if input.PublishStatus != "" && !input.PublishStatus.Valid() {
		return apperrors.ErrAnnouncementInvalidStatus
	}
	if input.DaysBeforeStart < 0 {
		return apperrors.ValidationError("จำนวนวันก่อนเริ่มแสดงต้องไม่น้อยกว่า 0")
	}
	if input.StartsAt != nil && input.EndsAt != nil && !input.StartsAt.Before(*input.EndsAt) {
		return apperrors.ValidationError("เวลาเริ่มแสดงต้องมาก่อนเวลาสิ้นสุด")
	}
	if input.CTAURL != "" && !validCTAURL(input.CTAURL) {
		return apperrors.ValidationError("CTA URL ไม่ถูกต้อง")
	}
	if input.Type == domain.TypeExamSchedule {
		if input.ExamTrackID == nil {
			return apperrors.ValidationError("กรุณาเลือกรายการสอบ")
		}
		if _, err := parseExamDate(input.ExamDate); err != nil {
			return apperrors.ValidationError("กรุณาระบุวันสอบให้ถูกต้อง")
		}
	}
	return nil
}

func validCTAURL(value string) bool {
	if strings.HasPrefix(value, "/") {
		return !strings.HasPrefix(value, "//")
	}
	parsed, err := url.ParseRequestURI(value)
	return err == nil && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

func parseExamDate(value *string) (*time.Time, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, errors.New("exam date is required")
	}
	parsed, err := time.Parse("2006-01-02", *value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
