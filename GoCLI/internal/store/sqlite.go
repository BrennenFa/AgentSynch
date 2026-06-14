package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"agentsynch/internal/objects"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS tasks (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    title        TEXT NOT NULL,
    description  TEXT,
    status       TEXT NOT NULL DEFAULT 'available',
    claimed_by   TEXT,
    claimed_at   TEXT,
    created_at   TEXT NOT NULL,
    finished_at  TEXT,
    output       TEXT,
    error        TEXT,
    plan         TEXT,
    heartbeat_at TEXT,
    attempts     INTEGER NOT NULL DEFAULT 0
);

// DAG task dependency table
CREATE TABLE IF NOT EXISTS task_dependencies (
    task_id       INTEGER NOT NULL REFERENCES tasks(id),
    depends_on_id INTEGER NOT NULL REFERENCES tasks(id),
    PRIMARY KEY (task_id, depends_on_id)
);`

func Open() (*sql.DB, error) {

	// issue if cannot find correct dir
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not find home directory: %w", err)
	}

	// issue if cannot create dir
	dir := filepath.Join(home, ".agentsynch")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("could not create %s: %w", dir, err)
	}

	// validate whether
	dbPath := filepath.Join(dir, "tasks.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("could not open database: %w", err)
	}

	// WAL mode allows concurrent readers alongside a writer
	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("could not set WAL mode: %w", err)
	}

	// enforces foreign key dependency w dag
	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("could not enable foreign keys: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("could not initialize schema: %w", err)
	}

	// migrate pre-existing DBs; SQLite returns "duplicate column name" for already-added columns
	db.Exec(`ALTER TABLE tasks ADD COLUMN heartbeat_at TEXT`)
	db.Exec(`ALTER TABLE tasks ADD COLUMN attempts INTEGER NOT NULL DEFAULT 0`)

	return db, nil
}


func AddTask(db *sql.DB, task objects.Task, deps []int64) (int64, error) {
	if len(deps) > 0 {
		task.Status = "blocked"
	}

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// insert tasks based on the given items
	result, err := tx.Exec(
		`INSERT INTO tasks (title, description, status, plan, claimed_by, claimed_at, created_at, finished_at, output, error)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.Title, task.Description, task.Status, task.Plan,
		task.ClaimedBy, task.ClaimedAt, task.CreatedAt,
		task.FinishedAt, task.Output, task.Error,
	)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	for _, depID := range deps {
		_, err := tx.Exec(
			`INSERT INTO task_dependencies (task_id, depends_on_id) VALUES (?, ?)`,
			id, depID,
		)
		if err != nil {
			return 0, fmt.Errorf("invalid dependency task-%d: %w", depID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
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

	return n1 + n2, nil
}

// HeartbeatTask stamps the current time onto a claimed task to signal the agent is alive.
func HeartbeatTask(db *sql.DB, id int64) error {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := db.Exec(
		`UPDATE tasks SET heartbeat_at = ? WHERE id = ? AND status = 'claimed'`,
		now, id,
	)
	return validateResults(result, err, id, "claimed")
}

func ClaimTask(db *sql.DB, agentID string) (*objects.Task, error) {
	// accquire db lock
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// find the first available task
	var t objects.Task
	err = tx.QueryRow(`SELECT id, title, description, status, created_at FROM tasks WHERE status = 'available' ORDER BY id LIMIT 1`).
		Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// claim it and increment the attempt counter
	claimedAt := time.Now().UTC().Format(time.RFC3339)
	_, err = tx.Exec(
		`UPDATE tasks SET status = 'claimed', claimed_by = ?, claimed_at = ?, attempts = attempts + 1 WHERE id = ?`,
		agentID, claimedAt, t.ID,
	)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	t.Status = "claimed"
	t.ClaimedBy = &agentID
	t.ClaimedAt = &claimedAt
	return &t, nil
}

func FinishTask(db *sql.DB, id int64, output string) error {
	// transition to validating -- a validator agent must approve before finished
	finishedAt := time.Now().UTC().Format(time.RFC3339)
	result, err := db.Exec(
		`UPDATE tasks SET status = 'validating', finished_at = ?, output = ? WHERE id = ? AND status = 'claimed'`,
		finishedAt, output, id,
	)
	return validateResults(result, err, id, "claimed")
}

func ValidateTask(db *sql.DB, id int64, approve bool, reason string) error {
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if approve {
		// mark finished and unblock any dependent tasks
		result, err := tx.Exec(
			`UPDATE tasks SET status = 'finished' WHERE id = ? AND status = 'validating'`,
			id,
		)
		if err := validateResults(result, err, id, "validating"); err != nil {
			return err
		}

		_, err = tx.Exec(`
			UPDATE tasks
			SET status = 'available'
			WHERE status = 'blocked'
			  AND id NOT IN (
			      SELECT d.task_id
			      FROM task_dependencies d
			      JOIN tasks dep ON dep.id = d.depends_on_id
			      WHERE dep.status != 'finished'
			  )
		`)
		if err != nil {
			return err
		}
	} else {
		// reject -- send back to available so an agent can reclaim and redo it
		result, err := tx.Exec(
			`UPDATE tasks SET status = 'available', claimed_by = NULL, claimed_at = NULL, finished_at = NULL, error = ? WHERE id = ? AND status = 'validating'`,
			reason, id,
		)
		if err := validateResults(result, err, id, "validating"); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func ErrorTask(db *sql.DB, id int64, errMsg string) error {
	// TODO -- Add a way to check go back and solve errors

	finishedAt := time.Now().UTC().Format(time.RFC3339)
	result, err := db.Exec(
		`UPDATE tasks SET status = 'error', finished_at = ?, error = ? WHERE id = ? AND status = 'claimed'`,
		finishedAt, errMsg, id,
	)
	return validateResults(result, err, id, "claimed")
}

func WritePlan(db *sql.DB, id int64, plan string) error {

	// add in a plan for how the task will be completed

	
	result, err := db.Exec(
		`UPDATE tasks SET plan = ? WHERE id = ? AND status = 'claimed'`,
		plan, id,
	)
	return validateResults(result, err, id, "claimed")
}

// validateResults checks that an Exec affected exactly one row.
func validateResults(result sql.Result, err error, id int64, status string) error {
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("task-%d not found or not %s", id, status)
	}
	return nil
}

func GetTask(db *sql.DB, id int64) (*objects.Task, error) {
	var t objects.Task
	err := db.QueryRow(`SELECT id, title, description, status, plan, claimed_by, claimed_at, created_at, finished_at, output, error FROM tasks WHERE id = ?`, id).Scan(
		&t.ID, &t.Title, &t.Description, &t.Status, &t.Plan,
		&t.ClaimedBy, &t.ClaimedAt, &t.CreatedAt,
		&t.FinishedAt, &t.Output, &t.Error,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}



func ListTasks(db *sql.DB) ([]objects.Task, error) {
	// query the rows needed
	rows, err := db.Query(`SELECT id, title, description, status, plan, claimed_by, claimed_at, created_at, finished_at, output, error FROM tasks ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []objects.Task
	idxByID := make(map[int64]int)

	// package tasks into a struct
	for rows.Next() {
		var t objects.Task
		err := rows.Scan(
			&t.ID, &t.Title, &t.Description, &t.Status, &t.Plan,
			&t.ClaimedBy, &t.ClaimedAt, &t.CreatedAt,
			&t.FinishedAt, &t.Output, &t.Error,
		)
		if err != nil {
			return nil, err
		}
		idxByID[t.ID] = len(tasks)
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	depRows, err := db.Query(`SELECT task_id, depends_on_id FROM task_dependencies ORDER BY task_id, depends_on_id`)
	if err != nil {
		return nil, err
	}
	defer depRows.Close()

	for depRows.Next() {
		var taskID, dependsOnID int64
		if err := depRows.Scan(&taskID, &dependsOnID); err != nil {
			return nil, err
		}
		if idx, ok := idxByID[taskID]; ok {
			tasks[idx].Dependencies = append(tasks[idx].Dependencies, dependsOnID)
		}
	}
	return tasks, depRows.Err()
}
