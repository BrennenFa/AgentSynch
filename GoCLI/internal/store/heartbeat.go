package store

import (
	"database/sql"
	"fmt"
	"time"
)

// HeartbeatTask stamps the current time onto a claimed task to signal the agent is alive.
func HeartbeatTask(db *sql.DB, id int64) error {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := db.Exec(
		`UPDATE tasks SET heartbeat_at = ? WHERE id = ? AND status = 'claimed'`,
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
		return fmt.Errorf("task-%d not found or not claimed", id)
	}
	return nil
}
