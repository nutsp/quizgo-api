package usecase

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"virtual-exam-api/internal/accesslog/domain"
	accessrepo "virtual-exam-api/internal/accesslog/repository"
	"virtual-exam-api/internal/common/pagination"
)

type LogInput struct {
	UserID    *uuid.UUID
	Email     string
	EventType string
	Success   bool
	IPAddress string
	UserAgent string
	Message   string
	Metadata  map[string]any
}

type Logger struct {
	repo accessrepo.Repository
}

func NewLogger(repo accessrepo.Repository) *Logger {
	return &Logger{repo: repo}
}

func (l *Logger) Log(ctx context.Context, input LogInput) {
	entry := &domain.AccessLog{
		ID:        uuid.New(),
		UserID:    input.UserID,
		Email:     input.Email,
		EventType: input.EventType,
		Success:   input.Success,
		IPAddress: input.IPAddress,
		UserAgent: input.UserAgent,
		Message:   input.Message,
		Metadata:  input.Metadata,
		CreatedAt: time.Now().UTC(),
	}
	if err := l.repo.Create(ctx, entry); err != nil {
		log.Printf("access log failed: %v", err)
	}
}

type AccessLogListResponse = pagination.PaginatedList[AccessLogResponse]

type AccessLogResponse struct {
	ID        string         `json:"id"`
	UserID    *string        `json:"user_id,omitempty"`
	Email     string         `json:"email,omitempty"`
	EventType string         `json:"event_type"`
	Success   bool           `json:"success"`
	IPAddress string         `json:"ip_address,omitempty"`
	UserAgent string         `json:"user_agent,omitempty"`
	Message   string         `json:"message,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt string         `json:"created_at"`
}

type AdminUseCase struct {
	repo accessrepo.Repository
}

func NewAdminUseCase(repo accessrepo.Repository) *AdminUseCase {
	return &AdminUseCase{repo: repo}
}

func (uc *AdminUseCase) List(ctx context.Context, filter accessrepo.AccessLogFilter) (*AccessLogListResponse, error) {
	items, total, err := uc.repo.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	resp := make([]AccessLogResponse, len(items))
	for i, item := range items {
		resp[i] = toResponse(item)
	}
	page, limit := pagination.Sanitize(filter.Page, filter.Limit)
	result := pagination.NewList(resp, page, limit, total)
	return &result, nil
}

func toResponse(item domain.AccessLog) AccessLogResponse {
	var userID *string
	if item.UserID != nil {
		s := item.UserID.String()
		userID = &s
	}
	return AccessLogResponse{
		ID:        item.ID.String(),
		UserID:    userID,
		Email:     item.Email,
		EventType: item.EventType,
		Success:   item.Success,
		IPAddress: item.IPAddress,
		UserAgent: item.UserAgent,
		Message:   item.Message,
		Metadata:  item.Metadata,
		CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339),
	}
}
