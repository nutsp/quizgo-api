package repository

import (
	"time"

	"github.com/google/uuid"
)

type SeasonModel struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	ExamTrackID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:leaderboard_seasons_exam_track_id_year_month_key,priority:1"`
	Year        int       `gorm:"not null;uniqueIndex:leaderboard_seasons_exam_track_id_year_month_key,priority:2"`
	Month       int       `gorm:"not null;uniqueIndex:leaderboard_seasons_exam_track_id_year_month_key,priority:3"`
	StartsAt    time.Time `gorm:"not null"`
	EndsAt      time.Time `gorm:"not null"`
	Status      string    `gorm:"type:varchar(20);not null"`
	FinalizedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (SeasonModel) TableName() string { return "leaderboard_seasons" }

type SeasonExamSetModel struct {
	SeasonID  uuid.UUID `gorm:"type:uuid;primaryKey"`
	ExamSetID uuid.UUID `gorm:"type:uuid;primaryKey"`
	JoinedAt  time.Time `gorm:"not null"`
	StoppedAt *time.Time
}

func (SeasonExamSetModel) TableName() string { return "leaderboard_season_exam_sets" }

type ScoreModel struct {
	SeasonID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	ExamSetID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	AttemptID       uuid.UUID `gorm:"type:uuid;not null"`
	Points          float64   `gorm:"type:numeric(6,1);not null"`
	DurationSeconds int       `gorm:"not null"`
	AchievedAt      time.Time `gorm:"not null"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (ScoreModel) TableName() string { return "leaderboard_scores" }

type EntryModel struct {
	SeasonID             uuid.UUID `gorm:"type:uuid;primaryKey;index:leaderboard_entries_rank_idx,priority:1"`
	UserID               uuid.UUID `gorm:"type:uuid;primaryKey"`
	TotalPoints          float64   `gorm:"type:numeric(10,1);not null;index:leaderboard_entries_rank_idx,priority:2,sort:desc"`
	CompletedExamSets    int       `gorm:"not null;index:leaderboard_entries_rank_idx,priority:3,sort:desc"`
	TotalDurationSeconds int64     `gorm:"not null;index:leaderboard_entries_rank_idx,priority:4,sort:asc"`
	ScoreAchievedAt      time.Time `gorm:"not null;index:leaderboard_entries_rank_idx,priority:5,sort:asc"`
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func (EntryModel) TableName() string { return "leaderboard_entries" }

type AwardModel struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	SeasonID  uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:leaderboard_awards_season_id_user_id_rank_key,priority:1"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:leaderboard_awards_season_id_user_id_rank_key,priority:2"`
	Rank      int       `gorm:"not null;uniqueIndex:leaderboard_awards_season_id_user_id_rank_key,priority:3"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (AwardModel) TableName() string { return "leaderboard_awards" }

type ProjectionFailureModel struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	AttemptID  uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:leaderboard_projection_failures_attempt_id_key"`
	RetryCount int       `gorm:"not null;default:0"`
	LastError  string    `gorm:"type:text;not null"`
	ResolvedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (ProjectionFailureModel) TableName() string { return "leaderboard_projection_failures" }
