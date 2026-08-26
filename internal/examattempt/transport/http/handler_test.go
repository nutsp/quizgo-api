package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"virtual-exam-api/internal/examattempt/quota"
	"virtual-exam-api/internal/examattempt/usecase"
	"virtual-exam-api/internal/middleware"
)

type statusQuota struct{}

func (statusQuota) Status(context.Context, uuid.UUID, time.Time) (quota.Status, error) {
	return quota.Status{Limit: 1, Used: 0, Remaining: 1, ResetAt: time.Date(2026, 7, 17, 17, 0, 0, 0, time.UTC)}, nil
}

func (statusQuota) WithSubmission(context.Context, uuid.UUID, time.Time, func() error) error {
	return nil
}

func TestGetDailyQuotaStatus(t *testing.T) {
	uc := usecase.NewExamAttemptUseCase(nil, nil, nil, nil, nil, nil, nil, nil, statusQuota{}, nil, nil, nil)
	handler := NewHandler(uc)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/daily-attempt-quota", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(middleware.UserIDKey, uuid.New())

	if err := handler.GetDailyQuotaStatus(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"remaining":1`) {
		t.Fatalf("response = %d %s", rec.Code, rec.Body.String())
	}
}
