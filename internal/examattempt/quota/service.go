package quota

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	ErrLimitReached = errors.New("free daily attempt limit reached")
	ErrBusy         = errors.New("daily submission is already in progress")
)

const (
	lockTTL = 30 * time.Second
)

var bangkokLocation = mustBangkokLocation()

type Store interface {
	GetUsed(ctx context.Context, key string) (used int, found bool, err error)
	SetUsed(ctx context.Context, key string, used int, ttl time.Duration) error
	AcquireLock(ctx context.Context, key, token string, ttl time.Duration) (bool, error)
	ReleaseLock(ctx context.Context, key, token string) error
}

type SubmissionHistory interface {
	CountSubmittedBetween(ctx context.Context, userID uuid.UUID, from, to time.Time) (int, error)
}

type Status struct {
	Limit            int        `json:"limit"`
	Used             int        `json:"used"`
	Remaining        int        `json:"remaining"`
	Unlimited        bool       `json:"unlimited"`
	ResetAt          time.Time  `json:"resetAt"`
	PremiumExpiresAt *time.Time `json:"premiumExpiresAt,omitempty"`
}

func (s Status) MarshalJSON() ([]byte, error) {
	if s.Unlimited {
		return json.Marshal(struct {
			Unlimited        bool       `json:"unlimited"`
			PremiumExpiresAt *time.Time `json:"premiumExpiresAt,omitempty"`
		}{Unlimited: true, PremiumExpiresAt: s.PremiumExpiresAt})
	}
	type statusAlias Status
	return json.Marshal(statusAlias(s))
}

type Service struct {
	store   Store
	history SubmissionHistory
	limit   int
}

func NewService(store Store, history SubmissionHistory, limit int) *Service {
	if limit < 1 {
		limit = 1
	}
	return &Service{store: store, history: history, limit: limit}
}

func (s *Service) Status(ctx context.Context, userID uuid.UUID, now time.Time) (Status, error) {
	start, resetAt := dayBounds(now)
	status := Status{Limit: s.limit, Remaining: s.limit, ResetAt: resetAt}
	used, found, err := s.store.GetUsed(ctx, MarkerKey(userID, now))
	if err != nil {
		return Status{}, err
	}
	if !found {
		used, err = s.history.CountSubmittedBetween(ctx, userID, start, resetAt)
		if err != nil {
			return Status{}, err
		}
		if used > 0 {
			if err := s.store.SetUsed(ctx, MarkerKey(userID, now), used, resetAt.Sub(now)); err != nil {
				return Status{}, err
			}
		}
	}
	if used < 0 {
		used = 0
	}
	status.Used = used
	status.Remaining = max(0, s.limit-used)
	return status, nil
}

func (s *Service) WithSubmission(ctx context.Context, userID uuid.UUID, now time.Time, submit func() error) error {
	token := uuid.NewString()
	lockKey := LockKey(userID, now)
	acquired, err := s.store.AcquireLock(ctx, lockKey, token, lockTTL)
	if err != nil {
		return err
	}
	if !acquired {
		return ErrBusy
	}
	defer func() { _ = s.store.ReleaseLock(context.WithoutCancel(ctx), lockKey, token) }()

	status, err := s.Status(ctx, userID, now)
	if err != nil {
		return err
	}
	if status.Remaining == 0 {
		return ErrLimitReached
	}
	if err := submit(); err != nil {
		return err
	}
	return s.store.SetUsed(ctx, MarkerKey(userID, now), status.Used+1, status.ResetAt.Sub(now))
}

func MarkerKey(userID uuid.UUID, now time.Time) string {
	return fmt.Sprintf("quota:free-attempt:%s:%s", userID, now.In(bangkokLocation).Format("2006-01-02"))
}

func LockKey(userID uuid.UUID, now time.Time) string {
	return fmt.Sprintf("lock:free-attempt:%s:%s", userID, now.In(bangkokLocation).Format("2006-01-02"))
}

func dayBounds(now time.Time) (time.Time, time.Time) {
	local := now.In(bangkokLocation)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, bangkokLocation)
	return start.UTC(), start.AddDate(0, 0, 1).UTC()
}

func mustBangkokLocation() *time.Location {
	location, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		panic(err)
	}
	return location
}
