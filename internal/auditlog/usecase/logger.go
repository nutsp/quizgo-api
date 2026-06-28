package usecase

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"virtual-exam-api/internal/auditlog/domain"
	auditrepo "virtual-exam-api/internal/auditlog/repository"
	"virtual-exam-api/internal/common/pagination"
)

type LogInput struct {
	ActorUserID  *uuid.UUID
	ActorEmail   string
	Action       string
	ResourceType string
	ResourceID   *uuid.UUID
	ResourceName string
	BeforeData   any
	AfterData    any
	IPAddress    string
	UserAgent    string
	Metadata     map[string]any
}

type Logger struct {
	repo auditrepo.Repository
}

func NewLogger(repo auditrepo.Repository) *Logger {
	return &Logger{repo: repo}
}

func (l *Logger) Log(ctx context.Context, input LogInput) {
	entry := &domain.AuditLog{
		ID:           uuid.New(),
		ActorUserID:  input.ActorUserID,
		ActorEmail:   input.ActorEmail,
		Action:       input.Action,
		ResourceType: input.ResourceType,
		ResourceID:   input.ResourceID,
		ResourceName: input.ResourceName,
		BeforeData:   SanitizeAuditData(input.BeforeData),
		AfterData:    SanitizeAuditData(input.AfterData),
		IPAddress:    input.IPAddress,
		UserAgent:    input.UserAgent,
		Metadata:     input.Metadata,
		CreatedAt:    time.Now().UTC(),
	}
	if err := l.repo.Create(ctx, entry); err != nil {
		log.Printf("audit log failed: %v", err)
	}
}

type AdminUseCase struct {
	repo auditrepo.Repository
}

func NewAdminUseCase(repo auditrepo.Repository) *AdminUseCase {
	return &AdminUseCase{repo: repo}
}

type AuditLogListResponse = pagination.PaginatedList[AuditLogResponse]

type AuditLogResponse struct {
	ID           string         `json:"id"`
	ActorUserID  *string        `json:"actor_user_id,omitempty"`
	ActorEmail   string         `json:"actor_email,omitempty"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceID   *string        `json:"resource_id,omitempty"`
	ResourceName string         `json:"resource_name,omitempty"`
	BeforeData   any            `json:"before_data,omitempty"`
	AfterData    any            `json:"after_data,omitempty"`
	IPAddress    string         `json:"ip_address,omitempty"`
	UserAgent    string         `json:"user_agent,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	CreatedAt    string         `json:"created_at"`
}

type DetailResponse = AuditLogResponse

func (uc *AdminUseCase) List(ctx context.Context, filter auditrepo.AuditLogFilter) (*AuditLogListResponse, error) {
	items, total, err := uc.repo.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	resp := make([]AuditLogResponse, len(items))
	for i, item := range items {
		resp[i] = toResponse(item)
	}
	page, limit := pagination.Sanitize(filter.Page, filter.Limit)
	result := pagination.NewList(resp, page, limit, total)
	return &result, nil
}

func (uc *AdminUseCase) Get(ctx context.Context, id uuid.UUID) (*DetailResponse, error) {
	item, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}
	resp := toResponse(*item)
	return &resp, nil
}

func toResponse(item domain.AuditLog) AuditLogResponse {
	var actorID *string
	if item.ActorUserID != nil {
		s := item.ActorUserID.String()
		actorID = &s
	}
	var resourceID *string
	if item.ResourceID != nil {
		s := item.ResourceID.String()
		resourceID = &s
	}
	return AuditLogResponse{
		ID:           item.ID.String(),
		ActorUserID:  actorID,
		ActorEmail:   item.ActorEmail,
		Action:       item.Action,
		ResourceType: item.ResourceType,
		ResourceID:   resourceID,
		ResourceName: item.ResourceName,
		BeforeData:   item.BeforeData,
		AfterData:    item.AfterData,
		IPAddress:    item.IPAddress,
		UserAgent:    item.UserAgent,
		Metadata:     item.Metadata,
		CreatedAt:    item.CreatedAt.UTC().Format(time.RFC3339),
	}
}
