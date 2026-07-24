package store

import (
	"context"
	"database/sql"
	"time"

	"agentsynch/internal/objects"
)

// Claim atomically claims the next available task, retrying up to 3 times on any error.
// TODO: add smarter retry logic (error-type filtering)
func Claim(db *sql.DB, agentID string, hostname string, pid int) (*objects.Task, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		task, err := claimAttempt(db, agentID, hostname, pid)
		if err == nil {
			return task, nil
		}
		lastErr = err
		time.Sleep(100 * time.Millisecond * (1 << attempt))
	}
	return nil, lastErr
}

// attempt a single claim
func claimAttempt(db *sql.DB, agentID string, hostname string, pid int) (*objects.Task, error) {
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// look for the oldest available task
	var task objects.Task
	var sameBranchInt int
	err = tx.QueryRow(
		`SELECT id, title, description, status, created_at, same_branch FROM tasks WHERE status = 'available' ORDER BY id LIMIT 1`,
	).Scan(&task.ID, &task.Title, &task.Description, &task.Status, &task.CreatedAt, &sameBranchInt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	task.SameBranch = sameBranchInt == 1
	claimedAt := time.Now().UTC().Format(time.RFC3339)

	_, err = tx.Exec(
		`UPDATE tasks SET status = 'claimed', claimed_by = ?, claimed_at = ?, attempts = attempts + 1, agent_hostname = ?, agent_pid = ? WHERE id = ?`,
		agentID, claimedAt, hostname, pid, task.ID,
	)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	task.Status = "claimed"
	task.ClaimedBy = &agentID
	task.ClaimedAt = &claimedAt
	return &task, nil
}
