package services

import (
	"math/rand/v2"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"course-api/config"
	"course-api/entities"
	"course-api/repository"
)

type CourseService struct {
	repo           repository.Repository
	bot            *BotService
	totalExercises int64
	scale          float64
	seed           int64
}

// NewCourseService panics on startup failures — fail fast rather than serving a broken state.
func NewCourseService(repo repository.Repository, v *viper.Viper) *CourseService {
	scale, seed, total, err := repo.GetMeta()
	if err != nil {
		logrus.WithError(err).Panic("failed to load dataset meta from database")
	}

	return &CourseService{
		repo: repo,
		bot: NewBotService(
			v.GetInt64(config.MIN_SINGLE_DWELL_MS),
			v.GetInt64(config.MIN_AVG_DWELL_MS),
			v.GetInt64(config.MAX_DAILY_LESSONS),
			v.GetInt(config.DWELL_WINDOW),
		),
		totalExercises: total,
		scale:          scale,
		seed:           seed,
	}
}

func (cs *CourseService) GetMeta() (scale float64, seed int64) {
	return cs.scale, cs.seed
}

func (cs *CourseService) Next(userID string) (entities.NextResult, error) {
	var res entities.NextResult
	err := cs.repo.WithTx(func(tx repository.TxRepo) error {
		p, err := tx.GetUserProgress(userID)
		if err != nil {
			return err
		}

		if p.IsActive {
			ex, err := tx.GetExercise(p.Cursor)
			if err != nil {
				return err
			}
			res.Exercise = &ex
			return nil
		}

		if p.Status == "bot" {
			maxID := p.Cursor
			if maxID < 1 {
				maxID = 1
			}
			ex, err := tx.GetExercise(rand.Int64N(maxID) + 1)
			if err != nil {
				return err
			}
			res.Exercise = &ex
			return nil
		}

		newCursor := p.Cursor + 1
		if newCursor > cs.totalExercises {
			res.CourseComplete = true
			return nil
		}

		affected, err := tx.AdvanceCursor(userID)
		if err != nil {
			return err
		}

		if affected == 0 {
			// Defensive fallback: GetUserProgress uses SELECT ... FOR UPDATE so
			// concurrent /next is already serialized per-user; under normal
			// operation this branch is unreachable. If it does fire (state
			// drift / operational anomaly), re-read with the same locking read
			// and serve whatever the latest active exercise is rather than
			// failing the request.
			p2, err := tx.GetUserProgress(userID)
			if err != nil {
				return err
			}
			if p2.IsActive {
				ex, err := tx.GetExercise(p2.Cursor)
				if err != nil {
					return err
				}
				res.Exercise = &ex
				return nil
			}
			logrus.WithField("user_id", userID).Warn("next: AdvanceCursor affected=0 but is_active=false; serving newCursor without RecordStart")
		} else {
			if err := tx.RecordStart(userID, newCursor%100, newCursor, time.Now().UnixMilli()); err != nil {
				return err
			}
		}

		ex, err := tx.GetExercise(newCursor)
		if err != nil {
			return err
		}
		res.Exercise = &ex
		return nil
	})
	if err != nil {
		return entities.NextResult{}, err
	}
	return res, nil
}

// Complete implements the state machine.
func (cs *CourseService) Complete(userID string) error {
	return cs.repo.WithTx(func(tx repository.TxRepo) error {
		affected, err := tx.MarkInactive(userID)
		if err != nil {
			return err
		}

		if affected == 0 {
			return nil
		}

		p, err := tx.GetUserProgress(userID)
		if err != nil {
			return err
		}

		slot := p.Cursor % 100
		if err := tx.RecordEndTime(userID, slot, time.Now().UnixMilli()); err != nil {
			return err
		}

		newStatus, err := cs.bot.DetectNewStatus(tx, userID, slot, p.Status)
		if err != nil {
			return err
		}

		if newStatus != p.Status {
			if err := tx.UpdateStatus(userID, newStatus); err != nil {
				return err
			}
		}

		return nil
	})
}

