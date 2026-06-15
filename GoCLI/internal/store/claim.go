package store

import (
	"context"
	"database/sql"
	"time"

	"agentsynch/internal/objects"
)

// Claim atomically claims the next task in a single serializable transaction.
// It prefers available (worker) tasks; only falls back to validating tasks when
// no available task exists — preventing validation starvation.
// Returns (task, validatorMode, error). Returns (nil, false, nil) if nothing to claim.
func Claim(db *sql.DB, agentID string) (*objects.Task, bool, error) {
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()

	// 1. Look for the oldest available (worker) task
	var workerTask objects.Task
	var sameBranchInt int
	workerErr := tx.QueryRow(
		`SELECT id, title, description, status, created_at, same_branch FROM tasks WHERE status = 'available' ORDER BY id LIMIT 1`,
	).Scan(&workerTask.ID, &workerTask.Title, &workerTask.Description, &workerTask.Status, &workerTask.CreatedAt, &sameBranchInt)

	if workerErr != nil && workerErr != sql.ErrNoRows {
		return nil, false, workerErr
	}

	if workerErr == nil {
		// Found an available task — claim it as worker
		workerTask.SameBranch = sameBranchInt == 1
		claimedAt := time.Now().UTC().Format(time.RFC3339)
		_, err = tx.Exec(
			`UPDATE tasks SET status = 'claimed', claimed_by = ?, claimed_at = ?, attempts = attempts + 1 WHERE id = ?`,
			agentID, claimedAt, workerTask.ID,
		)
		if err != nil {
			return nil, false, err
		}
		if err := tx.Commit(); err != nil {
			return nil, false, err
		}
		workerTask.Status = "claimed"
		workerTask.ClaimedBy = &agentID
		workerTask.ClaimedAt = &claimedAt
		return &workerTask, false, nil
	}

	// 2. No available task — look for an unclaimed validating task
	var valTask objects.Task
	valErr := tx.QueryRow(
		`SELECT id, title, description, status, created_at FROM tasks WHERE status = 'validating' AND validator_id IS NULL ORDER BY id LIMIT 1`,
	).Scan(&valTask.ID, &valTask.Title, &valTask.Description, &valTask.Status, &valTask.CreatedAt)

	if valErr == sql.ErrNoRows {
		return nil, false, nil
	}
	if valErr != nil {
		return nil, false, valErr
	}

	// Claim it as validator — does not increment attempts
	claimedAt := time.Now().UTC().Format(time.RFC3339)
	_, err = tx.Exec(
		`UPDATE tasks SET validator_id = ?, validation_claimed_at = ? WHERE id = ?`,
		agentID, claimedAt, valTask.ID,
	)
	if err != nil {
		return nil, false, err
	}
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}

	valTask.ValidatorID = &agentID
	valTask.ValidationClaimedAt = &claimedAt
	return &valTask, true, nil
}
