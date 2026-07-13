package repository

import (
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

func TestSeasonModelDeclaresMigrationChecks(t *testing.T) {
	t.Parallel()

	season := parseModelSchema(t, &SeasonModel{})
	checks := season.ParseCheckConstraints()

	assertCheckConstraint(t, checks, "leaderboard_seasons_month_check", "month BETWEEN 1 AND 12")
	assertCheckConstraint(t, checks, "leaderboard_seasons_status_check", "status IN ('active', 'finalized')")
}

func TestSeasonExamSetModelDeclaresIntervalKeys(t *testing.T) {
	t.Parallel()

	model := parseModelSchema(t, &SeasonExamSetModel{})
	if model.PrioritizedPrimaryField == nil || model.PrioritizedPrimaryField.Name != "ID" {
		t.Fatalf("primary key = %v, want ID", model.PrioritizedPrimaryField)
	}

	indexes := model.ParseIndexes()
	assertUniqueIndex(
		t,
		indexes,
		"leaderboard_season_exam_sets_interval_key",
		"",
		"SeasonID", "ExamSetID", "JoinedAt",
	)
	assertUniqueIndex(
		t,
		indexes,
		"leaderboard_season_exam_sets_one_open_idx",
		"stopped_at IS NULL",
		"SeasonID", "ExamSetID",
	)
}

func TestLeaderboardModelsDeclareMigrationForeignKeys(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		model      any
		relation   string
		constraint string
		references string
		onDelete   string
	}{
		{"season exam track", &SeasonModel{}, "ExamTrack", "leaderboard_seasons_exam_track_id_fkey", "exam_tracks", "NO ACTION"},
		{"season exam set season", &SeasonExamSetModel{}, "Season", "leaderboard_season_exam_sets_season_id_fkey", "leaderboard_seasons", "CASCADE"},
		{"season exam set exam set", &SeasonExamSetModel{}, "ExamSet", "leaderboard_season_exam_sets_exam_set_id_fkey", "exam_sets", "NO ACTION"},
		{"score season", &ScoreModel{}, "Season", "leaderboard_scores_season_id_fkey", "leaderboard_seasons", "CASCADE"},
		{"score user", &ScoreModel{}, "User", "leaderboard_scores_user_id_fkey", "users", "NO ACTION"},
		{"score exam set", &ScoreModel{}, "ExamSet", "leaderboard_scores_exam_set_id_fkey", "exam_sets", "NO ACTION"},
		{"score attempt", &ScoreModel{}, "Attempt", "leaderboard_scores_attempt_id_fkey", "exam_attempts", "NO ACTION"},
		{"entry season", &EntryModel{}, "Season", "leaderboard_entries_season_id_fkey", "leaderboard_seasons", "CASCADE"},
		{"entry user", &EntryModel{}, "User", "leaderboard_entries_user_id_fkey", "users", "NO ACTION"},
		{"award season", &AwardModel{}, "Season", "leaderboard_awards_season_id_fkey", "leaderboard_seasons", "CASCADE"},
		{"award user", &AwardModel{}, "User", "leaderboard_awards_user_id_fkey", "users", "NO ACTION"},
		{"projection failure attempt", &ProjectionFailureModel{}, "Attempt", "leaderboard_projection_failures_attempt_id_fkey", "exam_attempts", "NO ACTION"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			model := parseModelSchema(t, testCase.model)
			relation, ok := model.Relationships.Relations[testCase.relation]
			if !ok {
				t.Fatalf("missing %s relation", testCase.relation)
			}

			constraint := relation.ParseConstraint()
			if constraint == nil {
				t.Fatalf("missing %s constraint", testCase.relation)
			}
			if constraint.Name != testCase.constraint {
				t.Errorf("constraint name = %q, want %q", constraint.Name, testCase.constraint)
			}
			if constraint.ReferenceSchema.Table != testCase.references {
				t.Errorf("references table = %q, want %q", constraint.ReferenceSchema.Table, testCase.references)
			}
			if constraint.OnDelete != testCase.onDelete {
				t.Errorf("on delete = %q, want %q", constraint.OnDelete, testCase.onDelete)
			}
		})
	}
}

func parseModelSchema(t *testing.T, model any) *schema.Schema {
	t.Helper()

	parsed, err := schema.Parse(model, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatalf("parse model schema: %v", err)
	}
	return parsed
}

func assertCheckConstraint(t *testing.T, checks map[string]schema.CheckConstraint, name, expression string) {
	t.Helper()

	check, ok := checks[name]
	if !ok {
		t.Fatalf("missing %s check constraint", name)
	}
	if check.Constraint != expression {
		t.Errorf("check expression = %q, want %q", check.Constraint, expression)
	}
}

func assertUniqueIndex(t *testing.T, indexes map[string]schema.Index, name, where string, fieldNames ...string) {
	t.Helper()

	index, ok := indexes[name]
	if !ok {
		t.Fatalf("missing %s index", name)
	}
	if index.Class != "UNIQUE" {
		t.Errorf("%s class = %q, want UNIQUE", name, index.Class)
	}
	if index.Where != where {
		t.Errorf("%s where = %q, want %q", name, index.Where, where)
	}
	if len(index.Fields) != len(fieldNames) {
		t.Fatalf("%s fields = %d, want %d", name, len(index.Fields), len(fieldNames))
	}
	for i, fieldName := range fieldNames {
		if index.Fields[i].Name != fieldName {
			t.Errorf("%s field %d = %q, want %q", name, i, index.Fields[i].Name, fieldName)
		}
	}
}
