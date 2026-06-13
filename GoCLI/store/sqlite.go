package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"agentsynch/objects"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS tasks (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    title       TEXT NOT NULL,
    description TEXT,
    status      TEXT NOT NULL DEFAULT 'available',
    claimed_by  TEXT,
    claimed_at  TEXT,
    created_at  TEXT NOT NULL,
    finished_at TEXT,
    output      TEXT,
    error       TEXT,
    plan        TEXT
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

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("could not initialize schema: %w", err)
	}

	return db, nil
}


func AddTask(db *sql.DB, task objects.Task) (int64, error) {

	// insert tasks based on the given items
	result, err := db.Exec(
		`INSERT INTO tasks (title, description, status, plan, claimed_by, claimed_at, created_at, finished_at, output, error)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.Title, task.Description, task.Status, task.Plan,
		task.ClaimedBy, task.ClaimedAt, task.CreatedAt,
		task.FinishedAt, task.Output, task.Error,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
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

	// claim it
	claimedAt := time.Now().UTC().Format(time.RFC3339)
	_, err = tx.Exec(
		`UPDATE tasks SET status = 'claimed', claimed_by = ?, claimed_at = ? WHERE id = ?`,
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
	finishedAt := time.Now().UTC().Format(time.RFC3339)
	result, err := db.Exec(
		`UPDATE tasks SET status = 'finished', finished_at = ?, output = ? WHERE id = ? AND status = 'claimed'`,
		finishedAt, output, id,
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

func ErrorTask(db *sql.DB, id int64, errMsg string) error {
	// TODO -- Add a way to check go back and solve errors

	finishedAt := time.Now().UTC().Format(time.RFC3339)
	result, err := db.Exec(
		`UPDATE tasks SET status = 'error', finished_at = ?, error = ? WHERE id = ? AND status = 'claimed'`,
		finishedAt, errMsg, id,
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

func WritePlan(db *sql.DB, id int64, plan string) error {
	result, err := db.Exec(
		`UPDATE tasks SET plan = ? WHERE id = ? AND status = 'claimed'`,
		plan, id,
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

func ListTasks(db *sql.DB) ([]objects.Task, error) {
	// query the rows needed
	rows, err := db.Query(`SELECT id, title, description, status, plan, claimed_by, claimed_at, created_at, finished_at, output, error FROM tasks ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []objects.Task

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
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}
