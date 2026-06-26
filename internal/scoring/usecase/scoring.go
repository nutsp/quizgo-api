package usecase

import (
	"sort"
	"time"

	"github.com/google/uuid"
	qdomain "virtual-exam-api/internal/question/domain"
	scoringdomain "virtual-exam-api/internal/scoring/domain"
)

type ScoringUseCase struct{}

func NewScoringUseCase() *ScoringUseCase {
	return &ScoringUseCase{}
}

type ScoreInput struct {
	Answers        []AnswerInput
	CorrectChoices map[uuid.UUID]string
	TotalQuestions int
	PassingScore   int
	StartedAt      time.Time
	SubmittedAt    time.Time
}

type AnswerInput struct {
	QuestionID        uuid.UUID
	QuestionNo        int
	SelectedChoiceKey *string
	SubjectName       string
	Score             float64
}

func (uc *ScoringUseCase) Calculate(input ScoreInput) scoringdomain.ScoreResult {
	correct := 0
	wrong := 0
	unanswered := 0
	totalScore := float64(input.TotalQuestions)

	for _, a := range input.Answers {
		if a.SelectedChoiceKey == nil || *a.SelectedChoiceKey == "" {
			unanswered++
			continue
		}
		if correctKey, ok := input.CorrectChoices[a.QuestionID]; ok && *a.SelectedChoiceKey == correctKey {
			correct++
		} else {
			wrong++
		}
	}

	score := float64(correct)
	scorePercent := 0.0
	if totalScore > 0 {
		scorePercent = (score / totalScore) * 100
	}

	duration := int(input.SubmittedAt.Sub(input.StartedAt).Seconds())
	if duration < 0 {
		duration = 0
	}

	return scoringdomain.ScoreResult{
		Score:           score,
		TotalScore:      totalScore,
		ScorePercent:    scorePercent,
		CorrectCount:    correct,
		WrongCount:      wrong,
		UnansweredCount: unanswered,
		DurationSeconds: duration,
		Passed:          int(scorePercent) >= input.PassingScore,
	}
}

func (uc *ScoringUseCase) SubjectBreakdown(answers []AnswerInput, correctChoices map[uuid.UUID]string) []scoringdomain.SubjectScore {
	subjectStats := map[string]*scoringdomain.SubjectScore{}

	for _, a := range answers {
		name := a.SubjectName
		if name == "" {
			name = "ทั่วไป"
		}
		if _, ok := subjectStats[name]; !ok {
			subjectStats[name] = &scoringdomain.SubjectScore{SubjectName: name}
		}
		subjectStats[name].Total++
		if a.SelectedChoiceKey == nil || *a.SelectedChoiceKey == "" {
			subjectStats[name].Unanswered++
			continue
		}
		if correctKey, ok := correctChoices[a.QuestionID]; ok && *a.SelectedChoiceKey == correctKey {
			subjectStats[name].Correct++
		} else {
			subjectStats[name].Wrong++
		}
	}

	out := make([]scoringdomain.SubjectScore, 0, len(subjectStats))
	for _, s := range subjectStats {
		if s.Total > 0 {
			s.ScorePercent = (float64(s.Correct) / float64(s.Total)) * 100
		}
		out = append(out, *s)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].SubjectName < out[j].SubjectName
	})
	return out
}

func (uc *ScoringUseCase) WeaknessAnalysis(breakdown []scoringdomain.SubjectScore, topN int) []scoringdomain.SubjectScore {
	if topN <= 0 {
		topN = 3
	}
	sorted := make([]scoringdomain.SubjectScore, len(breakdown))
	copy(sorted, breakdown)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].ScorePercent == sorted[j].ScorePercent {
			return sorted[i].SubjectName < sorted[j].SubjectName
		}
		return sorted[i].ScorePercent < sorted[j].ScorePercent
	})
	if len(sorted) > topN {
		sorted = sorted[:topN]
	}
	return sorted
}

func BuildAnswerInputs(
	setQuestions []qdomain.ExamSetQuestion,
	answers []AnswerInput,
) []AnswerInput {
	questionSubjects := map[uuid.UUID]string{}
	for _, sq := range setQuestions {
		if sq.Question != nil && sq.Question.Subject != nil {
			questionSubjects[sq.QuestionID] = sq.Question.Subject.Name
		}
	}
	for i := range answers {
		if answers[i].SubjectName == "" {
			answers[i].SubjectName = questionSubjects[answers[i].QuestionID]
		}
	}
	return answers
}

func ToPublicChoices(choices []qdomain.Choice) []qdomain.ChoicePublic {
	out := make([]qdomain.ChoicePublic, len(choices))
	for i, c := range choices {
		out[i] = qdomain.ChoicePublic{
			ChoiceKey:   c.ChoiceKey,
			ChoiceLabel: c.ChoiceLabel,
			ChoiceText:  c.ChoiceText,
		}
	}
	return out
}
