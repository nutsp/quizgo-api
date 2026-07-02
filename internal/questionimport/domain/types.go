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
	"tags",
	"explanation",
	"difficulty",
	"status",
	"question_type",
	"content_format",
	"question_image",
	"choice_a_image",
	"choice_b_image",
	"choice_c_image",
	"choice_d_image",
	"explanation_image",
}

type ImportQuestionRow struct {
	RowNumber           int    `json:"row_number"`
	SubjectCode         string `json:"subject_code"`
	Tags                string `json:"tags,omitempty"`
	QuestionType        string `json:"question_type,omitempty"`
	ContentFormat       string `json:"content_format,omitempty"`
	QuestionText        string `json:"question_text"`
	QuestionImage       string `json:"question_image,omitempty"`
	QuestionImageURL    string `json:"question_image_url,omitempty"`
	ChoiceA             string `json:"choice_a"`
	ChoiceAImage        string `json:"choice_a_image,omitempty"`
	ChoiceAImageURL     string `json:"choice_a_image_url,omitempty"`
	ChoiceB             string `json:"choice_b"`
	ChoiceBImage        string `json:"choice_b_image,omitempty"`
	ChoiceBImageURL     string `json:"choice_b_image_url,omitempty"`
	ChoiceC             string `json:"choice_c"`
	ChoiceCImage        string `json:"choice_c_image,omitempty"`
	ChoiceCImageURL     string `json:"choice_c_image_url,omitempty"`
	ChoiceD             string `json:"choice_d"`
	ChoiceDImage        string `json:"choice_d_image,omitempty"`
	ChoiceDImageURL     string `json:"choice_d_image_url,omitempty"`
	CorrectChoice       string `json:"correct_choice"`
	Explanation         string `json:"explanation,omitempty"`
	ExplanationImage    string `json:"explanation_image,omitempty"`
	ExplanationImageURL string `json:"explanation_image_url,omitempty"`
	Difficulty          string `json:"difficulty,omitempty"`
	Status              string `json:"status,omitempty"`
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
	ID                  uuid.UUID
	ImportJobID         uuid.UUID
	RowNumber           int
	SubjectCode         string
	Tags                string
	QuestionType        string
	ContentFormat       string
	QuestionText        string
	QuestionImage       string
	QuestionImageURL    string
	ChoiceA             string
	ChoiceAImage        string
	ChoiceAImageURL     string
	ChoiceB             string
	ChoiceBImage        string
	ChoiceBImageURL     string
	ChoiceC             string
	ChoiceCImage        string
	ChoiceCImageURL     string
	ChoiceD             string
	ChoiceDImage        string
	ChoiceDImageURL     string
	CorrectChoice       string
	Explanation         string
	ExplanationImage    string
	ExplanationImageURL string
	Difficulty          string
	Status              string
	Valid               bool
	Errors              []string
	Warnings            []string
	CreatedAt           time.Time
}
