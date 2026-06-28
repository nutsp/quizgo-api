package domain

import (
	"time"

	"github.com/google/uuid"
)

type QuestionTag struct {
	ID          uuid.UUID
	Name        string
	Code        string
	Description string
	Color       string
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type TagRef struct {
	ID    uuid.UUID
	Name  string
	Code  string
	Color string
}
