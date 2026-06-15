package store

import (
	"database/sql"
	"fmt"
	"time"
)

// HeartbeatTask stamps the current time onto a claimed or validating task to signal the agent is alive.
func HeartbeatTask(db *sql.DB, id int64) error {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := db.Exec(
		`UPDATE tasks SET heartbeat_at = ? WHERE id = ? AND status IN ('claimed', 'validating')`,
		now, id,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("task-%d not found or not claimed/validating", id)
	}
	return nil
}

// ReapZombies reclaims stale claimed tasks (zombie agents). Tasks that have been
// attempted fewer than maxAttempts times are reset to 'available' for retry; tasks
// that have exhausted their attempts are marked 'error'. Returns total rows affected.
func ReapZombies(db *sql.DB, timeout time.Duration) (int64, error) {

	const maxAttempts = 3

	threshold := time.Now().UTC().Add(-timeout).Format(time.RFC3339)
	zombieWhere := `
		status = 'claimed'
		AND (
		    (heartbeat_at IS NOT NULL AND heartbeat_at < ?)
		    OR (heartbeat_at IS NULL AND claimed_at < ?)
		)`

	// tasks under the attempt limit go back to available for retry
	r1, err := db.Exec(`
		UPDATE tasks
		SET status = 'available', claimed_by = NULL, claimed_at = NULL, heartbeat_at = NULL
		WHERE `+zombieWhere+` AND attempts < ?`,
		threshold, threshold, maxAttempts,
	)
	if err != nil {
		return 0, err
	}
	n1, err := r1.RowsAffected()
	if err != nil {
		return 0, err
	}

	// tasks that have hit the limit are marked error (no more retries)
	r2, err := db.Exec(`
		UPDATE tasks
		SET status = 'error', claimed_by = NULL, claimed_at = NULL, heartbeat_at = NULL,
		    error = 'max attempts reached: agent did not complete task'
		WHERE `+zombieWhere+` AND attempts >= ?`,
		threshold, threshold, maxAttempts,
	)
	if err != nil {
		return 0, err
	}
	n2, err := r2.RowsAffected()
	if err != nil {
		return 0, err
	}

	// reap timed-out validators: clear validator fields but keep status 'validating'
	// (work is done, just needs a new validator assigned)
	r3, err := db.Exec(`
		UPDATE tasks
		SET validator_id = NULL, validation_claimed_at = NULL
		WHERE status = 'validating'
		  AND validator_id IS NOT NULL
		  AND (
		    (heartbeat_at IS NOT NULL AND heartbeat_at < ?)
		    OR (heartbeat_at IS NULL AND validation_claimed_at < ?)
		  )`,
		threshold, threshold,
	)
	if err != nil {
		return 0, err
	}
	n3, err := r3.RowsAffected()
	if err != nil {
		return 0, err
	}

	return n1 + n2 + n3, nil
}
