package store

import (
	"context"
	"database/sql"
	"time"

	"agentsynch/internal/objects"
)

// Claim atomically claims the next available task in a single serializable transaction.
// Returns (task, error). Returns (nil, nil) if no tasks are available.
func Claim(db *sql.DB, agentID string, hostname string, pid int) (*objects.Task, error) {
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Look for the oldest available task
	var workerTask objects.Task
	workerErr := tx.QueryRow(
		`SELECT id, title, description, status, created_at FROM tasks WHERE status = 'available' ORDER BY id LIMIT 1`,
	).Scan(&workerTask.ID, &workerTask.Title, &workerTask.Description, &workerTask.Status, &workerTask.CreatedAt)

	if workerErr == sql.ErrNoRows {
		return nil, nil
	}
	if workerErr != nil {
		return nil, workerErr
	}

	// Found an available task — claim it
	claimedAt := time.Now().UTC().Format(time.RFC3339)
	_, err = tx.Exec(
		`UPDATE tasks SET status = 'claimed', claimed_by = ?, claimed_at = ?, attempts = attempts + 1, agent_hostname = ?, agent_pid = ? WHERE id = ?`,
		agentID, claimedAt, hostname, pid, workerTask.ID,
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
