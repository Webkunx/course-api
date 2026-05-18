package repository

import (
	"errors"

	"course-api/entities"
)

var ErrNotFound = errors.New("not found")

// TxRepo exposes all write-path operations within a single transaction.
type TxRepo interface {
	GetUserProgress(userID string) (entities.UserProgress, error)
	GetExercise(exerciseID int64) (entities.Exercise, error)
	// AdvanceCursor atomically increments cursor and sets is_active=1.
	// Returns affected rows (0 = lost is_active race, caller should ErrRetry).
	AdvanceCursor(userID string) (int64, error)
	RecordStart(userID string, slot, exerciseID, startMs int64) error
	// MarkInactive atomically clears is_active. Returns affected rows (0 = idempotent call).
	MarkInactive(userID string) (int64, error)
	RecordEndTime(userID string, slot, endMs int64) error
	GetDwellStats(userID string, slot int64, window int) (entities.DwellStats, error)
	IncrementDaily(userID, day string) (int64, error)
	UpdateStatus(userID, status string) error
}

// Repository is the top-level store abstraction used by the server.
type Repository interface {
	WithTx(fn func(TxRepo) error) error
	CreateUser(userID string, createdAt int64) error
	UserExists(userID string) (bool, error)
	GetMeta() (scale float64, seed int64, total int64, err error)
}
