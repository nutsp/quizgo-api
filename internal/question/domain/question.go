package domain

import (
	"time"

	"github.com/google/uuid"
)

type Subject struct {
	ID          uuid.UUID
	Code        string
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Question struct {
	ID           uuid.UUID
	SubjectID    uuid.UUID
	QuestionText string
	Explanation  string
	Difficulty   string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Subject      *SubjectRef
	Choices      []Choice
}

type SubjectRef struct {
	Code string
	Name string
}

type Choice struct {
	ID          uuid.UUID
	QuestionID  uuid.UUID
	ChoiceKey   string
	ChoiceLabel string
	ChoiceText  string
	IsCorrect   bool
}

type ChoicePublic struct {
	ChoiceKey   string `json:"choice_key"`
	ChoiceLabel string `json:"choice_label"`
	ChoiceText  string `json:"choice_text"`
}

type ExamSetQuestion struct {
	ID         uuid.UUID
	ExamSetID  uuid.UUID
	QuestionID uuid.UUID
	QuestionNo int
	Score      float64
	Question   *Question
}

type QuestionForExam struct {
	QuestionNo   int            `json:"question_no"`
	QuestionID   string         `json:"question_id"`
	QuestionText string         `json:"question_text"`
	SubjectName  string         `json:"subject_name,omitempty"`
	Choices      []ChoicePublic `json:"choices"`
}

type QuestionForReview struct {
	QuestionNo         int            `json:"question_no"`
	QuestionID         string         `json:"question_id"`
	QuestionText       string         `json:"question_text"`
	Choices            []ChoicePublic `json:"choices"`
	SelectedChoiceKey  *string        `json:"selected_choice_key"`
	CorrectChoiceKey   string         `json:"correct_choice_key"`
	IsCorrect          bool           `json:"is_correct"`
	Explanation        string         `json:"explanation"`
	Subject            string         `json:"subject"`
}

const (
	ChoiceA = "A"
	ChoiceB = "B"
	ChoiceC = "C"
	ChoiceD = "D"
)

var ValidChoiceKeys = map[string]string{
	ChoiceA: "ก",
	ChoiceB: "ข",
	ChoiceC: "ค",
	ChoiceD: "ง",
}

func IsValidChoiceKey(key string) bool {
	_, ok := ValidChoiceKeys[key]
	return ok
}
