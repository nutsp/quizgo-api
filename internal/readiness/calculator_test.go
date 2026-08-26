package readiness

import "testing"

func TestCalculatorCollectsEvidenceBeforeShowingScore(t *testing.T) {
	calculator := NewCalculator(DefaultConfig())

	result := calculator.Calculate(Input{
		BlueprintReady:         true,
		Sections:               []SectionEvidence{{WeightPercent: 100, Answered: 9, Correct: 7, RecentAnswered: 9, RecentCorrect: 7}},
		DurationLimitSeconds:   3600,
		AverageDurationSeconds: 3000,
	})

	if result.Status != StatusCollecting {
		t.Fatalf("status = %q, want %q", result.Status, StatusCollecting)
	}
	if result.Score != nil {
		t.Fatalf("collecting result must not expose a score, got %.1f", *result.Score)
	}
	if result.Evidence.AnsweredQuestions != 9 || result.Evidence.RequiredQuestions != 20 {
		t.Fatalf("unexpected evidence: %+v", result.Evidence)
	}
}

func TestCalculatorProducesExplainableWeightedScore(t *testing.T) {
	calculator := NewCalculator(DefaultConfig())

	result := calculator.Calculate(Input{
		BlueprintReady: true,
		Sections: []SectionEvidence{
			{WeightPercent: 60, Answered: 10, Correct: 8, RecentAnswered: 10, RecentCorrect: 8},
			{WeightPercent: 40, Answered: 10, Correct: 5, RecentAnswered: 10, RecentCorrect: 4},
		},
		DurationLimitSeconds:   3600,
		AverageDurationSeconds: 3000,
	})

	if result.Status != StatusReady || result.Score == nil {
		t.Fatalf("expected ready score, got %+v", result)
	}
	assertNear(t, *result.Score, 79.2)
	assertNear(t, result.Components.WeightedAccuracy, 68)
	assertNear(t, result.Components.Coverage, 100)
	assertNear(t, result.Components.RecentRetention, 60)
	assertNear(t, result.Components.TimeManagement, 100)
	if result.CalculationVersion != "readiness-v1" {
		t.Fatalf("calculation version = %q", result.CalculationVersion)
	}
	if len(result.Explanations) == 0 {
		t.Fatal("ready result must include plain-language explanations")
	}
}

func TestCalculatorDoesNotInventScoreWithoutReviewedBlueprint(t *testing.T) {
	calculator := NewCalculator(DefaultConfig())
	result := calculator.Calculate(Input{BlueprintReady: false})

	if result.Status != StatusUnavailable || result.Score != nil {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func assertNear(t *testing.T, got, want float64) {
	t.Helper()
	if got < want-0.01 || got > want+0.01 {
		t.Fatalf("got %.2f, want %.2f", got, want)
	}
}
