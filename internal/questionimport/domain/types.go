package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	JobStatusPreview   = "preview"
	JobStatusImported  = "imported"
	JobStatusConfirmed = "already_imported"

	MaxFileSize = 5 * 1024 * 1024 // 5MB
)

var RequiredColumns = []string{
	"subject_code",
	"question_text",
	"choice_a",
	"choice_b",
	"choice_c",
	"choice_d",
	"correct_choice",
}

var OptionalColumns = []string{
	"explanation",
	"difficulty",
	"status",
}

type ImportQuestionRow struct {
	RowNumber     int    `json:"row_number"`
	SubjectCode   string `json:"subject_code"`
	QuestionText  string `json:"question_text"`
	ChoiceA       string `json:"choice_a"`
	ChoiceB       string `json:"choice_b"`
	ChoiceC       string `json:"choice_c"`
	ChoiceD       string `json:"choice_d"`
	CorrectChoice string `json:"correct_choice"`
	Explanation   string `json:"explanation,omitempty"`
	Difficulty    string `json:"difficulty,omitempty"`
	Status        string `json:"status,omitempty"`
}

type ImportPreviewRow struct {
	RowNumber int               `json:"row_number"`
	Data      ImportQuestionRow `json:"data"`
	Valid     bool              `json:"valid"`
	Errors    []string          `json:"errors"`
	Warnings  []string          `json:"warnings"`
}

type ImportPreviewResult struct {
	ImportID    uuid.UUID          `json:"import_id"`
	Filename    string             `json:"filename"`
	TotalRows   int                `json:"total_rows"`
	ValidRows   int                `json:"valid_rows"`
	InvalidRows int                `json:"invalid_rows"`
	Rows        []ImportPreviewRow `json:"rows"`
}

type ImportConfirmInput struct {
	ImportID            uuid.UUID `json:"import_id"`
	ImportOnlyValidRows bool      `json:"import_only_valid_rows"`
}

type ImportConfirmResult struct {
	ImportID          uuid.UUID `json:"import_id"`
	Status            string    `json:"status"`
	ImportedQuestions int       `json:"imported_questions"`
	SkippedRows       int       `json:"skipped_rows"`
	FailedRows        int       `json:"failed_rows,omitempty"`
}

type ImportJob struct {
	ID                uuid.UUID
	AdminUserID       uuid.UUID
	Filename          string
	Status            string
	TotalRows         int
	ValidRows         int
	InvalidRows       int
	ImportedQuestions int
	SkippedRows       int
	FailedRows        int
	CreatedAt         time.Time
	ConfirmedAt       *time.Time
}

type ImportJobRow struct {
	ID            uuid.UUID
	ImportJobID   uuid.UUID
	RowNumber     int
	SubjectCode   string
	QuestionText  string
	ChoiceA       string
	ChoiceB       string
	ChoiceC       string
	ChoiceD       string
	CorrectChoice string
	Explanation   string
	Difficulty    string
	Status        string
	Valid         bool
	Errors        []string
	Warnings      []string
	CreatedAt     time.Time
}
