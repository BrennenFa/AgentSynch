package store

import (
	"context"
	"database/sql"
	"time"

	"agentsynch/internal/objects"
)

// Claim atomically claims the next available task in a serializable transaction.
// Returns (task, error). Returns (nil, nil) if nothing to claim.
func Claim(db *sql.DB, agentID string) (*objects.Task, error) {
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Look for the oldest available task
	var workerTask objects.Task
	var sameBranchInt int
	workerErr := tx.QueryRow(
		`SELECT id, title, description, status, created_at, same_branch FROM tasks WHERE status = 'available' ORDER BY id LIMIT 1`,
	).Scan(&workerTask.ID, &workerTask.Title, &workerTask.Description, &workerTask.Status, &workerTask.CreatedAt, &sameBranchInt)

	if workerErr != nil && workerErr != sql.ErrNoRows {
		return nil, workerErr
	}

	if workerErr == sql.ErrNoRows {
		return nil, nil
	}

	workerTask.SameBranch = sameBranchInt == 1
	claimedAt := time.Now().UTC().Format(time.RFC3339)
	_, err = tx.Exec(
		`UPDATE tasks SET status = 'claimed', claimed_by = ?, claimed_at = ?, attempts = attempts + 1 WHERE id = ?`,
		agentID, claimedAt, workerTask.ID,
	)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	workerTask.Status = "claimed"
	workerTask.ClaimedBy = &agentID
	workerTask.ClaimedAt = &claimedAt
	return &workerTask, nil
}
