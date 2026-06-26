package domain

import (
	"time"

	"github.com/google/uuid"
)

type AvailableFilter struct {
	Query           string
	SubjectID       uuid.UUID
	Difficulty      string
	Status          string
	ExcludeAssigned bool
	Page            int
	Limit           int
}

type SubjectRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type AvailableQuestion struct {
	ID               uuid.UUID
	QuestionText     string
	Subject          *SubjectRef
	Difficulty       string
	Status           string
	CorrectChoiceKey string
	CreatedAt        time.Time
	AlreadyAssigned  bool
}

type AssignedQuestion struct {
	QuestionID   uuid.UUID
	QuestionNo   int
	Score        float64
	QuestionText string
	Subject      *SubjectRef
	Difficulty   string
	Status       string
}

type BulkAddResult struct {
	ExamSetID        uuid.UUID
	AddedCount       int
	SkippedCount     int
	TotalQuestions   int
	AddedQuestions   []AddedQuestion
	SkippedQuestions []SkippedQuestion
}

type AddedQuestion struct {
	QuestionID uuid.UUID
	QuestionNo int
}

type SkippedQuestion struct {
	QuestionID uuid.UUID
	Reason     string
}

type ReorderItem struct {
	QuestionID uuid.UUID
	QuestionNo int
}

type ExamSetSummary struct {
	ID              uuid.UUID
	Code            string
	Title           string
	TotalQuestions  int
	DurationMinutes int
	PassingScore    int
}
