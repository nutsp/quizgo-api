package usecase

import (
	"context"

	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/cache"
	"virtual-exam-api/internal/examset/domain"
	examsetrepo "virtual-exam-api/internal/examset/repository"
	trackdomain "virtual-exam-api/internal/examtrack/domain"
	trackrepo "virtual-exam-api/internal/examtrack/repository"
)

type ExamTrackUseCase struct {
	tracks       trackrepo.Repository
	examSets     examsetrepo.Repository
	contentCache cache.CacheService
}

func NewExamTrackUseCase(
	tracks trackrepo.Repository,
	examSets examsetrepo.Repository,
	contentCache cache.CacheService,
) *ExamTrackUseCase {
	if contentCache == nil {
		contentCache = cache.Noop()
	}
	return &ExamTrackUseCase{
		tracks:       tracks,
		examSets:     examSets,
		contentCache: contentCache,
	}
}

func (uc *ExamTrackUseCase) List(ctx context.Context) ([]trackdomain.ExamTrackSummary, error) {
	key := cache.ExamTracksList()
	var cached []trackdomain.ExamTrackSummary
	if ok, _ := uc.contentCache.GetJSON(ctx, key, &cached); ok {
		return cached, nil
	}

	tracks, err := uc.tracks.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]trackdomain.ExamTrackSummary, len(tracks))
	for i := range tracks {
		out[i] = tracks[i].ToSummary()
	}

	_ = uc.contentCache.SetJSON(ctx, key, out, cache.TTLExamTracksList)
	_ = uc.contentCache.AddIndex(ctx, cache.IndexExamTracks(), key, cache.TTLExamTracksList+cache.TTLIndexBuffer)

	return out, nil
}

func (uc *ExamTrackUseCase) GetByCode(ctx context.Context, code string) (*trackdomain.ExamTrackSummary, error) {
	tracks, err := uc.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range tracks {
		if tracks[i].Code == code {
			summary := tracks[i]
			return &summary, nil
		}
	}
	return nil, apperrors.ErrExamTrackNotFound
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

	hash := cache.HashExamSetListFilter(filter)
	key := cache.ExamSetsByTrack(trackCode, hash)
	var cached domain.PaginatedResult
	if ok, _ := uc.contentCache.GetJSON(ctx, key, &cached); ok {
		return &cached, nil
	}

	result, err := uc.examSets.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	_ = uc.contentCache.SetJSON(ctx, key, result, cache.TTLExamSetsByTrack)
	_ = uc.contentCache.AddIndex(ctx, cache.IndexExamSetsList(), key, cache.TTLExamSetsByTrack+cache.TTLIndexBuffer)

	return result, nil
}
