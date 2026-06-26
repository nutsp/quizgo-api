package domain

import "github.com/google/uuid"

type ScoreResult struct {
	Score           float64
	TotalScore      float64
	ScorePercent    float64
	CorrectCount    int
	WrongCount      int
	UnansweredCount int
	DurationSeconds int
	Passed          bool
}

type AnswerScore struct {
	QuestionID uuid.UUID
	QuestionNo int
	IsCorrect  bool
	Score      float64
}

type SubjectScore struct {
	SubjectName  string
	Correct      int
	Wrong        int
	Unanswered   int
	Total        int
	ScorePercent float64
}
