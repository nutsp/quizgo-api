package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	leaderboardrepo "virtual-exam-api/internal/leaderboard/repository"
	leaderboardusecase "virtual-exam-api/internal/leaderboard/usecase"
)

func TestRunRequiresExactlyOneSelectionModeAndRejectsFutureMonth(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 14, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing mode", args: []string{"-year", "2026", "-month", "7"}, wantErr: "exactly one"},
		{name: "conflicting modes", args: []string{"-track-code", "gpor", "-all-active-tracks", "-year", "2026", "-month", "7"}, wantErr: "exactly one"},
		{name: "future month", args: []string{"-track-code", "gpor", "-year", "2026", "-month", "8"}, wantErr: "future month"},
		{name: "invalid month", args: []string{"-track-code", "gpor", "-year", "2026", "-month", "13"}, wantErr: "month must be between 1 and 12"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var stderr bytes.Buffer
			factoryCalled := false
			exitCode := run(t.Context(), tc.args, &bytes.Buffer{}, &stderr, now, func(context.Context) (trackSource, seasonReconciler, error) {
				factoryCalled = true
				return nil, nil, nil
			})
			if exitCode == 0 {
				t.Fatal("run() exit code = 0, want non-zero")
			}
			if factoryCalled {
				t.Fatal("dependency factory called for invalid arguments")
			}
			if !strings.Contains(stderr.String(), tc.wantErr) {
				t.Errorf("stderr = %q, want %q", stderr.String(), tc.wantErr)
			}
		})
	}
}

func TestRunTrackCodePrintsSeasonSummary(t *testing.T) {
	t.Parallel()

	track := leaderboardrepo.ExamTrackContextRow{
		ID: uuid.MustParse("71000000-0000-0000-0000-000000000001"), Code: "gpor", Name: "Gpor",
	}
	source := &fakeTrackSource{byCode: map[string]*leaderboardrepo.ExamTrackContextRow{"gpor": &track}}
	reconciler := &fakeSeasonReconciler{summaries: map[uuid.UUID]leaderboardusecase.ReconcileSummary{
		track.ID: {
			SeasonID:   uuid.MustParse("72000000-0000-0000-0000-000000000002"),
			ScoreCount: 12,
			EntryCount: 7,
		},
	}}
	var stdout, stderr bytes.Buffer
	exitCode := run(
		t.Context(),
		[]string{"-track-code", "gpor", "-year", "2026", "-month", "7"},
		&stdout,
		&stderr,
		time.Date(2026, time.July, 14, 10, 0, 0, 0, time.UTC),
		func(context.Context) (trackSource, seasonReconciler, error) { return source, reconciler, nil },
	)
	if exitCode != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", exitCode, stderr.String())
	}
	if got, want := stdout.String(), "track=gpor season_id=72000000-0000-0000-0000-000000000002 scores=12 entries=7\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
	if len(reconciler.calls) != 1 {
		t.Fatalf("reconciliation calls = %d, want 1", len(reconciler.calls))
	}
	window, err := leaderboardMonthWindow(reconciler.calls[0].at)
	if err != nil || window.year != 2026 || window.month != 7 {
		t.Errorf("reconciliation month = %+v, error = %v, want 2026-07", window, err)
	}
}

func TestRunAllTracksContinuesAfterPerTrackFailureAndReturnsNonZero(t *testing.T) {
	t.Parallel()

	first := leaderboardrepo.ExamTrackContextRow{ID: uuid.New(), Code: "first"}
	second := leaderboardrepo.ExamTrackContextRow{ID: uuid.New(), Code: "second"}
	source := &fakeTrackSource{active: []leaderboardrepo.ExamTrackContextRow{first, second}}
	reconciler := &fakeSeasonReconciler{
		summaries: map[uuid.UUID]leaderboardusecase.ReconcileSummary{
			first.ID: {SeasonID: uuid.New(), ScoreCount: 3, EntryCount: 2},
		},
		errs: map[uuid.UUID]error{second.ID: errors.New("database unavailable")},
	}
	var stdout, stderr bytes.Buffer
	exitCode := run(
		t.Context(),
		[]string{"-all-active-tracks", "-year", "2026", "-month", "7"},
		&stdout,
		&stderr,
		time.Date(2026, time.July, 14, 10, 0, 0, 0, time.UTC),
		func(context.Context) (trackSource, seasonReconciler, error) { return source, reconciler, nil },
	)
	if exitCode == 0 {
		t.Fatal("run() exit code = 0, want non-zero")
	}
	if len(reconciler.calls) != 2 {
		t.Fatalf("reconciliation calls = %d, want both tracks", len(reconciler.calls))
	}
	if !strings.Contains(stdout.String(), "track=first season_id=") {
		t.Errorf("stdout = %q, want first track summary", stdout.String())
	}
	if !strings.Contains(stderr.String(), "track=second reconciliation failed: database unavailable") {
		t.Errorf("stderr = %q, want second track failure", stderr.String())
	}
}

type fakeTrackSource struct {
	byCode map[string]*leaderboardrepo.ExamTrackContextRow
	active []leaderboardrepo.ExamTrackContextRow
}

func (f *fakeTrackSource) FindActiveExamTrackByCode(_ context.Context, code string) (*leaderboardrepo.ExamTrackContextRow, error) {
	return f.byCode[code], nil
}

func (f *fakeTrackSource) ListActiveExamTracks(context.Context) ([]leaderboardrepo.ExamTrackContextRow, error) {
	return f.active, nil
}

type reconcileCall struct {
	trackID uuid.UUID
	at      time.Time
}

type fakeSeasonReconciler struct {
	summaries map[uuid.UUID]leaderboardusecase.ReconcileSummary
	errs      map[uuid.UUID]error
	calls     []reconcileCall
}

func (f *fakeSeasonReconciler) ReconcileSeason(
	_ context.Context,
	trackID uuid.UUID,
	at time.Time,
) (leaderboardusecase.ReconcileSummary, error) {
	f.calls = append(f.calls, reconcileCall{trackID: trackID, at: at})
	return f.summaries[trackID], f.errs[trackID]
}
