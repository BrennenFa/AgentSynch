package store

import (
	"context"
	"database/sql"
	"math/rand"
	"strings"
	"time"

	"agentsynch/internal/objects"
)

const claimMaxRetries = 5
const claimBaseDelay = 100 * time.Millisecond

// Claim atomically claims the next available task in a single serializable transaction.
// Retries with exponential backoff on SQLite busy/serialization errors.
// Returns (task, error). Returns (nil, nil) if no tasks are available.
func Claim(db *sql.DB, agentID string, hostname string, pid int) (*objects.Task, error) {
	for attempt := 0; attempt < claimMaxRetries; attempt++ {
		task, err := claimOnce(db, agentID, hostname, pid)
		if err == nil {
			return task, nil
		}
		// retry on SQLite busy or serialization failure
		if !isRetryable(err) || attempt == claimMaxRetries-1 {
			return nil, err
		}
		// exponential backoff with jitter: 100ms, 200ms, 400ms, 800ms ...
		delay := claimBaseDelay * (1 << attempt)
		jitter := time.Duration(rand.Int63n(int64(delay) / 2))
		time.Sleep(delay + jitter)
	}
	return nil, nil
}

func claimOnce(db *sql.DB, agentID string, hostname string, pid int) (*objects.Task, error) {
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

// isRetryable returns true for SQLite busy or serialization errors.
func isRetryable(err error) bool {
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "busy") || strings.Contains(s, "locked") || strings.Contains(s, "serializ")
}
