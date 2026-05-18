package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"

	"course-api/entities"
)

type mysqlRepo struct{ db *sql.DB }

// New wraps a *sql.DB with the Repository interface.
func New(db *sql.DB) Repository { return &mysqlRepo{db: db} }

func (r *mysqlRepo) WithTx(fn func(TxRepo) error) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	if err := fn(&mysqlTxRepo{tx: tx}); err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func (r *mysqlRepo) CreateUser(userID string, createdAt int64) error {
	_, err := r.db.Exec(`INSERT INTO user_progress (user_id, created_at) VALUES (?, ?)`, userID, createdAt)
	return err
}

func (r *mysqlRepo) UserExists(userID string) (bool, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM user_progress WHERE user_id=?`, userID).Scan(&count)
	return count > 0, err
}

func (r *mysqlRepo) GetMeta() (scale float64, seed int64, total int64, err error) {
	var totalStr, scaleStr, seedStr string
	if err = r.db.QueryRow(`SELECT value FROM meta WHERE meta_key='total_exercises'`).Scan(&totalStr); err != nil {
		return
	}
	_ = r.db.QueryRow(`SELECT value FROM meta WHERE meta_key='dataset_scale'`).Scan(&scaleStr)
	_ = r.db.QueryRow(`SELECT value FROM meta WHERE meta_key='dataset_seed'`).Scan(&seedStr)
	total, _ = strconv.ParseInt(totalStr, 10, 64)
	scale, _ = strconv.ParseFloat(scaleStr, 64)
	seed, _ = strconv.ParseInt(seedStr, 10, 64)
	return
}

// mysqlTxRepo implements TxRepo within a *sql.Tx.
type mysqlTxRepo struct{ tx *sql.Tx }

// GetUserProgress reads the user_progress row inside the current tx with
// SELECT ... FOR UPDATE. Two reasons this is the right semantic:
//  1. Reads always observe the latest committed row, not the tx snapshot
//     (InnoDB REPEATABLE READ + locking read). The /next lost-race recovery
//     and /complete's post-update read both rely on this.
//  2. It serializes concurrent /next calls for the same user_id, which is
//     the behavior we want (intra-account parallelism is part of the threat
//     model). Cross-user concurrency is unaffected — row-level locking only.
func (t *mysqlTxRepo) GetUserProgress(userID string) (entities.UserProgress, error) {
	var p entities.UserProgress
	var isActive int64
	err := t.tx.QueryRow(
		`SELECT status, `+"`cursor`"+`, is_active FROM user_progress WHERE user_id=? FOR UPDATE`, userID,
	).Scan(&p.Status, &p.Cursor, &isActive)
	if err == sql.ErrNoRows {
		return entities.UserProgress{}, ErrNotFound
	}
	if err != nil {
		return entities.UserProgress{}, err
	}
	p.IsActive = isActive == 1
	return p, nil
}

func (t *mysqlTxRepo) GetExercise(exerciseID int64) (entities.Exercise, error) {
	var content string
	err := t.tx.QueryRow(`SELECT content FROM exercises WHERE exercise_id=?`, exerciseID).Scan(&content)
	if err != nil {
		return entities.Exercise{}, fmt.Errorf("fetch exercise %d: %w", exerciseID, err)
	}
	var ex entities.Exercise
	if err := json.Unmarshal([]byte(content), &ex); err != nil {
		return entities.Exercise{}, fmt.Errorf("unmarshal exercise %d: %w", exerciseID, err)
	}
	return ex, nil
}

func (t *mysqlTxRepo) AdvanceCursor(userID string) (int64, error) {
	res, err := t.tx.Exec(
		"UPDATE user_progress SET `cursor`=`cursor`+1, is_active=1 WHERE user_id=? AND is_active=0",
		userID,
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (t *mysqlTxRepo) RecordStart(userID string, slot, exerciseID, startMs int64) error {
	_, err := t.tx.Exec(
		`REPLACE INTO user_completion (user_id, slot, exercise_id, start_time, end_time) VALUES (?, ?, ?, ?, NULL)`,
		userID, slot, exerciseID, startMs,
	)
	return err
}

func (t *mysqlTxRepo) MarkInactive(userID string) (int64, error) {
	res, err := t.tx.Exec(`UPDATE user_progress SET is_active=0 WHERE user_id=? AND is_active=1`, userID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (t *mysqlTxRepo) RecordEndTime(userID string, slot, endMs int64) error {
	_, err := t.tx.Exec(
		`UPDATE user_completion SET end_time=? WHERE user_id=? AND slot=?`,
		endMs, userID, slot,
	)
	return err
}

// GetDwellStats fetches single-lesson dwell and rolling-average dwell in one round trip.
func (t *mysqlTxRepo) GetDwellStats(userID string, slot int64, window int) (entities.DwellStats, error) {
	var singleMs sql.NullInt64
	var avgMs sql.NullFloat64
	err := t.tx.QueryRow(`
		SELECT
			(SELECT end_time - start_time
			 FROM user_completion
			 WHERE user_id=? AND slot=?) AS single_ms,
			(SELECT AVG(d)
			 FROM (SELECT end_time - start_time AS d
			       FROM user_completion
			       WHERE user_id=? AND end_time IS NOT NULL
			       ORDER BY start_time DESC
			       LIMIT ?) AS recent) AS avg_ms`,
		userID, slot, userID, window,
	).Scan(&singleMs, &avgMs)
	if err != nil {
		return entities.DwellStats{}, err
	}
	return entities.DwellStats{
		SingleMs: singleMs.Int64,
		AvgMs:    avgMs.Float64,
		HasAvg:   avgMs.Valid,
	}, nil
}

func (t *mysqlTxRepo) IncrementDaily(userID, day string) (int64, error) {
	if _, err := t.tx.Exec(
		`INSERT INTO user_daily (user_id, day, completed) VALUES (?, ?, 1)
		 ON DUPLICATE KEY UPDATE completed=completed+1`,
		userID, day,
	); err != nil {
		return 0, err
	}
	var completed int64
	err := t.tx.QueryRow(`SELECT completed FROM user_daily WHERE user_id=? AND day=?`, userID, day).Scan(&completed)
	return completed, err
}

func (t *mysqlTxRepo) UpdateStatus(userID, status string) error {
	_, err := t.tx.Exec(`UPDATE user_progress SET status=? WHERE user_id=?`, status, userID)
	return err
}

