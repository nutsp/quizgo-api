package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"virtual-exam-api/internal/config"
	"virtual-exam-api/internal/database"
	leaderboardrepo "virtual-exam-api/internal/leaderboard/repository"
	leaderboardusecase "virtual-exam-api/internal/leaderboard/usecase"
)

type trackSource interface {
	FindActiveExamTrackByCode(context.Context, string) (*leaderboardrepo.ExamTrackContextRow, error)
	ListActiveExamTracks(context.Context) ([]leaderboardrepo.ExamTrackContextRow, error)
}

type seasonReconciler interface {
	ReconcileSeason(context.Context, uuid.UUID, time.Time) (leaderboardusecase.ReconcileSummary, error)
}

type dependencyFactory func(context.Context) (trackSource, seasonReconciler, error)

type leaderboardMonth struct {
	year  int
	month int
}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr, time.Now().UTC(), newDependencies))
}

func run(
	ctx context.Context,
	args []string,
	stdout, stderr io.Writer,
	now time.Time,
	dependencies dependencyFactory,
) int {
	current, err := leaderboardMonthWindow(now)
	if err != nil {
		fmt.Fprintf(stderr, "resolve current leaderboard month: %v\n", err)
		return 1
	}

	flags := flag.NewFlagSet("leaderboard-reconcile", flag.ContinueOnError)
	flags.SetOutput(stderr)
	trackCode := flags.String("track-code", "", "active exam track code")
	year := flags.Int("year", current.year, "leaderboard season year")
	month := flags.Int("month", current.month, "leaderboard season month")
	allActiveTracks := flags.Bool("all-active-tracks", false, "reconcile every active exam track")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "unexpected positional arguments")
		return 2
	}

	*trackCode = strings.TrimSpace(*trackCode)
	hasTrackCode := *trackCode != ""
	if hasTrackCode == *allActiveTracks {
		fmt.Fprintln(stderr, "exactly one of -track-code or -all-active-tracks is required")
		return 2
	}
	if *year <= 0 {
		fmt.Fprintln(stderr, "year must be positive")
		return 2
	}
	if *month < 1 || *month > 12 {
		fmt.Fprintln(stderr, "month must be between 1 and 12")
		return 2
	}
	target := leaderboardMonth{year: *year, month: *month}
	if target.year > current.year || (target.year == current.year && target.month > current.month) {
		fmt.Fprintf(stderr, "future month %04d-%02d cannot be reconciled\n", target.year, target.month)
		return 2
	}

	location, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		fmt.Fprintf(stderr, "load Asia/Bangkok: %v\n", err)
		return 1
	}
	at := time.Date(target.year, time.Month(target.month), 1, 0, 0, 0, 0, location).UTC()

	tracks, reconciler, err := dependencies(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "initialize reconciliation: %v\n", err)
		return 1
	}
	selected, err := selectTracks(ctx, tracks, *trackCode, *allActiveTracks)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	exitCode := 0
	for _, track := range selected {
		summary, err := reconciler.ReconcileSeason(ctx, track.ID, at)
		if err != nil {
			fmt.Fprintf(stderr, "track=%s reconciliation failed: %v\n", track.Code, err)
			exitCode = 1
			continue
		}
		fmt.Fprintf(
			stdout,
			"track=%s season_id=%s scores=%d entries=%d\n",
			track.Code,
			summary.SeasonID,
			summary.ScoreCount,
			summary.EntryCount,
		)
	}
	return exitCode
}

func selectTracks(
	ctx context.Context,
	tracks trackSource,
	trackCode string,
	allActiveTracks bool,
) ([]leaderboardrepo.ExamTrackContextRow, error) {
	if allActiveTracks {
		rows, err := tracks.ListActiveExamTracks(ctx)
		if err != nil {
			return nil, fmt.Errorf("list active tracks: %w", err)
		}
		return rows, nil
	}
	row, err := tracks.FindActiveExamTrackByCode(ctx, trackCode)
	if err != nil {
		return nil, fmt.Errorf("find active track %q: %w", trackCode, err)
	}
	if row == nil {
		return nil, fmt.Errorf("active track %q was not found", trackCode)
	}
	return []leaderboardrepo.ExamTrackContextRow{*row}, nil
}

func leaderboardMonthWindow(at time.Time) (leaderboardMonth, error) {
	location, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		return leaderboardMonth{}, err
	}
	local := at.In(location)
	return leaderboardMonth{year: local.Year(), month: int(local.Month())}, nil
}

func newDependencies(context.Context) (trackSource, seasonReconciler, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}
	db, err := database.NewPostgres(cfg)
	if err != nil {
		return nil, nil, err
	}
	repo := leaderboardrepo.NewPostgresRepository(db)
	return repo, leaderboardusecase.NewLeaderboardUseCase(repo), nil
}
