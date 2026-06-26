package usecase

import (
	"context"

	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/examset/domain"
	examsetrepo "virtual-exam-api/internal/examset/repository"
	trackdomain "virtual-exam-api/internal/examtrack/domain"
	trackrepo "virtual-exam-api/internal/examtrack/repository"
)

type ExamTrackUseCase struct {
	tracks   trackrepo.Repository
	examSets examsetrepo.Repository
}

func NewExamTrackUseCase(tracks trackrepo.Repository, examSets examsetrepo.Repository) *ExamTrackUseCase {
	return &ExamTrackUseCase{tracks: tracks, examSets: examSets}
}

func (uc *ExamTrackUseCase) List(ctx context.Context) ([]trackdomain.ExamTrackSummary, error) {
	tracks, err := uc.tracks.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]trackdomain.ExamTrackSummary, len(tracks))
	for i := range tracks {
		out[i] = tracks[i].ToSummary()
	}
	return out, nil
}

func (uc *ExamTrackUseCase) GetByCode(ctx context.Context, code string) (*trackdomain.ExamTrackSummary, error) {
	track, err := uc.tracks.FindByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if track == nil || !track.IsActive {
		return nil, apperrors.ErrExamTrackNotFound
	}
	summary := track.ToSummary()
	return &summary, nil
}

func (uc *ExamTrackUseCase) ListExamSets(ctx context.Context, trackCode string, filter domain.ListFilter) (*domain.PaginatedResult, error) {
	track, err := uc.tracks.FindByCode(ctx, trackCode)
	if err != nil {
		return nil, err
	}
	if track == nil || !track.IsActive {
		return nil, apperrors.ErrExamTrackNotFound
	}

	filter.TrackID = track.ID
	filter.OnlyPublished = true
	return uc.examSets.List(ctx, filter)
}
