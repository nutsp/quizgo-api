package usecase

import (
	"context"
	"testing"

	"github.com/google/uuid"
	resultrepo "virtual-exam-api/internal/result/repository"
)

type readinessResultRepo struct {
	resultrepo.Repository
	track resultrepo.TrackRow
}

func (r readinessResultRepo) FindTrackByCode(context.Context, string) (*resultrepo.TrackRow, error) {
	return &r.track, nil
}

func (r readinessResultRepo) FindTrackByID(context.Context, uuid.UUID) (*resultrepo.TrackRow, error) {
	return &r.track, nil
}

func (readinessResultRepo) CountActiveExamSetsByTrack(context.Context, uuid.UUID) (int64, error) {
	return 1, nil
}

func (readinessResultRepo) GetTrackBestAttempts(context.Context, uuid.UUID, uuid.UUID) ([]resultrepo.BestAttemptRow, error) {
	return []resultrepo.BestAttemptRow{{ScorePercent: 70, PassingScore: 60}}, nil
}

func (readinessResultRepo) CountAttemptsByTrack(context.Context, uuid.UUID, uuid.UUID) (int64, error) {
	return 2, nil
}

func (readinessResultRepo) GetLatestAttemptScoreByTrack(context.Context, uuid.UUID, uuid.UUID) (float64, bool, error) {
	return 70, true, nil
}

func (readinessResultRepo) ListWeakSubjects(context.Context, uuid.UUID, *uuid.UUID, int) ([]resultrepo.WeakSubjectRow, error) {
	return nil, nil
}

func (readinessResultRepo) ListActiveExamSetsByTrack(context.Context, uuid.UUID) ([]resultrepo.ExamSetRow, error) {
	return nil, nil
}

func (readinessResultRepo) GetReadinessEvidence(context.Context, uuid.UUID, uuid.UUID) (*resultrepo.ReadinessEvidenceRow, error) {
	return &resultrepo.ReadinessEvidenceRow{
		TotalAnswered:          20,
		AverageDurationSeconds: 3000,
		Sections: []resultrepo.ReadinessSectionRow{
			{WeightPercent: 60, Answered: 10, Correct: 8, RecentAnswered: 10, RecentCorrect: 8},
			{WeightPercent: 40, Answered: 10, Correct: 5, RecentAnswered: 10, RecentCorrect: 4},
		},
	}, nil
}

func TestTrackDetailUsesExplainableReadinessInsteadOfBestScoreAlias(t *testing.T) {
	trackID := uuid.New()
	repo := readinessResultRepo{track: resultrepo.TrackRow{
		ID:                       trackID,
		Code:                     "agnostic-track",
		Name:                     "Any exam",
		BlueprintStatus:          "reviewed",
		BlueprintDurationMinutes: 60,
	}}
	uc := NewResultUseCase(repo, premiumChecker{active: true})

	got, err := uc.GetMyExamTrackResultDetail(context.Background(), uuid.New(), repo.track.Code)
	if err != nil {
		t.Fatal(err)
	}
	if got.Summary.Readiness.Status != "ready" || got.Summary.Readiness.Score == nil {
		t.Fatalf("readiness = %+v", got.Summary.Readiness)
	}
	if *got.Summary.Readiness.Score == got.Summary.AverageBestScorePercent {
		t.Fatal("readiness must not be an alias of average best score")
	}
	if got.Summary.Readiness.CalculationVersion != "readiness-v1" {
		t.Fatalf("calculation version = %q", got.Summary.Readiness.CalculationVersion)
	}
}
