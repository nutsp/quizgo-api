package repository

import (
	"time"

	"github.com/google/uuid"
	attemptrepo "virtual-exam-api/internal/examattempt/repository"
	examsetrepo "virtual-exam-api/internal/examset/repository"
	trackrepo "virtual-exam-api/internal/examtrack/repository"
	userrepo "virtual-exam-api/internal/user/repository"
)

type SeasonModel struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	ExamTrackID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:leaderboard_seasons_exam_track_id_year_month_key,priority:1"`
	Year        int       `gorm:"not null;uniqueIndex:leaderboard_seasons_exam_track_id_year_month_key,priority:2"`
	Month       int       `gorm:"not null;check:leaderboard_seasons_month_check,month BETWEEN 1 AND 12;uniqueIndex:leaderboard_seasons_exam_track_id_year_month_key,priority:3"`
	StartsAt    time.Time `gorm:"not null"`
	EndsAt      time.Time `gorm:"not null"`
	Status      string    `gorm:"type:varchar(20);not null;check:leaderboard_seasons_status_check,status IN ('active', 'finalized')"`
	FinalizedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time

	ExamTrack trackrepo.ExamTrackModel `gorm:"foreignKey:ExamTrackID;references:ID;constraint:leaderboard_seasons_exam_track_id_fkey,OnDelete:NO ACTION"`
}

func (SeasonModel) TableName() string { return "leaderboard_seasons" }

type ExamSetStopEventModel struct {
	ExamSetID uuid.UUID `gorm:"type:uuid;primaryKey"`
	StoppedAt time.Time `gorm:"primaryKey"`
	CreatedAt time.Time `gorm:"not null;default:now()"`

	ExamSet examsetrepo.ExamSetModel `gorm:"foreignKey:ExamSetID;references:ID;constraint:leaderboard_exam_set_stop_events_exam_set_id_fkey,OnDelete:NO ACTION"`
}

func (ExamSetStopEventModel) TableName() string { return "leaderboard_exam_set_stop_events" }

type SeasonExamSetModel struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey"`
	SeasonID  uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:leaderboard_season_exam_sets_interval_key,priority:1;uniqueIndex:leaderboard_season_exam_sets_one_open_idx,priority:1,where:stopped_at IS NULL"`
	ExamSetID uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:leaderboard_season_exam_sets_interval_key,priority:2;uniqueIndex:leaderboard_season_exam_sets_one_open_idx,priority:2,where:stopped_at IS NULL"`
	JoinedAt  time.Time  `gorm:"not null;uniqueIndex:leaderboard_season_exam_sets_interval_key,priority:3"`
	StoppedAt *time.Time `gorm:"check:leaderboard_season_exam_sets_interval_check,stopped_at IS NULL OR stopped_at >= joined_at"`

	Season  SeasonModel              `gorm:"foreignKey:SeasonID;references:ID;constraint:leaderboard_season_exam_sets_season_id_fkey,OnDelete:CASCADE"`
	ExamSet examsetrepo.ExamSetModel `gorm:"foreignKey:ExamSetID;references:ID;constraint:leaderboard_season_exam_sets_exam_set_id_fkey,OnDelete:NO ACTION"`
}

func (SeasonExamSetModel) TableName() string { return "leaderboard_season_exam_sets" }

type ScoreModel struct {
	SeasonID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	ExamSetID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	AttemptID       uuid.UUID `gorm:"type:uuid;not null"`
	Points          float64   `gorm:"type:numeric(6,1);not null;check:leaderboard_scores_points_check,points >= 0 AND points <= 100"`
	DurationSeconds int       `gorm:"not null;check:leaderboard_scores_duration_seconds_check,duration_seconds >= 0"`
	AchievedAt      time.Time `gorm:"not null"`
	CreatedAt       time.Time
	UpdatedAt       time.Time

	Season  SeasonModel                  `gorm:"foreignKey:SeasonID;references:ID;constraint:leaderboard_scores_season_id_fkey,OnDelete:CASCADE"`
	User    userrepo.UserModel           `gorm:"foreignKey:UserID;references:ID;constraint:leaderboard_scores_user_id_fkey,OnDelete:NO ACTION"`
	ExamSet examsetrepo.ExamSetModel     `gorm:"foreignKey:ExamSetID;references:ID;constraint:leaderboard_scores_exam_set_id_fkey,OnDelete:NO ACTION"`
	Attempt attemptrepo.ExamAttemptModel `gorm:"foreignKey:AttemptID;references:ID;constraint:leaderboard_scores_attempt_id_fkey,OnDelete:NO ACTION"`
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

	Season SeasonModel        `gorm:"foreignKey:SeasonID;references:ID;constraint:leaderboard_entries_season_id_fkey,OnDelete:CASCADE"`
	User   userrepo.UserModel `gorm:"foreignKey:UserID;references:ID;constraint:leaderboard_entries_user_id_fkey,OnDelete:NO ACTION"`
}

func (EntryModel) TableName() string { return "leaderboard_entries" }

type AwardModel struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	SeasonID  uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:leaderboard_awards_season_id_user_id_rank_key,priority:1"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:leaderboard_awards_season_id_user_id_rank_key,priority:2"`
	Rank      int       `gorm:"not null;uniqueIndex:leaderboard_awards_season_id_user_id_rank_key,priority:3"`
	CreatedAt time.Time
	UpdatedAt time.Time

	Season SeasonModel        `gorm:"foreignKey:SeasonID;references:ID;constraint:leaderboard_awards_season_id_fkey,OnDelete:CASCADE"`
	User   userrepo.UserModel `gorm:"foreignKey:UserID;references:ID;constraint:leaderboard_awards_user_id_fkey,OnDelete:NO ACTION"`
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

	Attempt attemptrepo.ExamAttemptModel `gorm:"foreignKey:AttemptID;references:ID;constraint:leaderboard_projection_failures_attempt_id_fkey,OnDelete:NO ACTION"`
}

func (ProjectionFailureModel) TableName() string { return "leaderboard_projection_failures" }
