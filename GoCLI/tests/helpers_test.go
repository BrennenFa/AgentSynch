package tests

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"agentsynch/internal/objects"
	_ "modernc.org/sqlite"
)

// newTestDB creates an isolated in-memory SQLite DB with the full schema.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()

	// Each test gets its own named in-memory DB so they don't share state.
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		t.Fatalf("WAL mode: %v", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		t.Fatalf("foreign keys: %v", err)
	}

	schema := `
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
    attempts     INTEGER NOT NULL DEFAULT 0,
    validator_id          TEXT,
    validation_claimed_at TEXT,
    same_branch  INTEGER NOT NULL DEFAULT 0,
    branch_name  TEXT,
    gh_url       TEXT
);
CREATE TABLE IF NOT EXISTS task_dependencies (
    task_id       INTEGER NOT NULL REFERENCES tasks(id),
    depends_on_id INTEGER NOT NULL REFERENCES tasks(id),
    PRIMARY KEY (task_id, depends_on_id)
);`

	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("schema: %v", err)
	}

	t.Cleanup(func() { db.Close() })
	return db
}

// seedTasks inserts n tasks with the given status.
// For "validating" tasks, finished_at is set and validator_id is left NULL.
func seedTasks(t *testing.T, db *sql.DB, n int, status string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	for i := 0; i < n; i++ {
		var err error
		if status == "validating" {
			_, err = db.Exec(
				`INSERT INTO tasks (title, description, status, created_at, finished_at) VALUES (?, ?, ?, ?, ?)`,
				fmt.Sprintf("task-%d", i), "test task", status, now, now,
			)
		} else {
			_, err = db.Exec(
				`INSERT INTO tasks (title, description, status, created_at) VALUES (?, ?, ?, ?)`,
				fmt.Sprintf("task-%d", i), "test task", status, now,
			)
		}
		if err != nil {
			t.Fatalf("seed task %d: %v", i, err)
		}
	}
}

// naiveClaim is the old two-transaction pattern — used as the control arm for
// the starvation comparison test. It tries available first in tx1, then
// validating in a separate tx2 if nothing was found.
func naiveClaim(db *sql.DB, agentID string) (*objects.Task, bool, error) {
	// Tx 1: claim oldest available task
	tx1, err := db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, false, err
	}
	defer tx1.Rollback()

	var task objects.Task
	err = tx1.QueryRow(
		`SELECT id, title, description, status, created_at FROM tasks WHERE status = 'available' ORDER BY id LIMIT 1`,
	).Scan(&task.ID, &task.Title, &task.Description, &task.Status, &task.CreatedAt)

	if err != nil && err != sql.ErrNoRows {
		return nil, false, err
	}

	if err == nil {
		claimedAt := time.Now().UTC().Format(time.RFC3339)
		_, execErr := tx1.Exec(
			`UPDATE tasks SET status = 'claimed', claimed_by = ?, claimed_at = ?, attempts = attempts + 1 WHERE id = ?`,
			agentID, claimedAt, task.ID,
		)
		if execErr != nil {
			return nil, false, execErr
		}
		if err := tx1.Commit(); err != nil {
			return nil, false, err
		}
		task.Status = "claimed"
		task.ClaimedBy = &agentID
		task.ClaimedAt = &claimedAt
		return &task, false, nil
	}
	tx1.Rollback()

	// Gap between transactions — the race window that allows starvation
	time.Sleep(time.Microsecond)

	// Tx 2: claim oldest unclaimed validating task
	tx2, err := db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, false, err
	}
	defer tx2.Rollback()

	var valTask objects.Task
	err = tx2.QueryRow(
		`SELECT id, title, description, status, created_at FROM tasks WHERE status = 'validating' AND validator_id IS NULL ORDER BY id LIMIT 1`,
	).Scan(&valTask.ID, &valTask.Title, &valTask.Description, &valTask.Status, &valTask.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	claimedAt := time.Now().UTC().Format(time.RFC3339)
	_, err = tx2.Exec(
		`UPDATE tasks SET validator_id = ?, validation_claimed_at = ? WHERE id = ?`,
		agentID, claimedAt, valTask.ID,
	)
	if err != nil {
		return nil, false, err
	}
	if err := tx2.Commit(); err != nil {
		return nil, false, err
	}
	valTask.ValidatorID = &agentID
	valTask.ValidationClaimedAt = &claimedAt
	return &valTask, true, nil
}

// MetricsResult holds the output of one scaling test run.
type MetricsResult struct {
	AgentCount         int     `json:"agent_count"`
	TotalTasks         int     `json:"total_tasks"`
	TasksCompleted     int     `json:"tasks_completed"`
	DurationMs         int64   `json:"duration_ms"`
	ThroughputPerSec   float64 `json:"throughput_per_sec"`
	SimTokensPerSec    float64 `json:"sim_tokens_per_sec"`
	ContentionErrors   int64   `json:"contention_errors"`
	ValidationsClaimed int     `json:"validations_claimed"`
}

// writeMetricsJSON appends results (with a timestamp) to GoCLI/tests/metrics.json.
func writeMetricsJSON(results []MetricsResult) error {
	type entry struct {
		Timestamp string          `json:"timestamp"`
		Results   []MetricsResult `json:"results"`
	}

	// Read existing file if present
	path := filepath.Join("metrics.json")
	var entries []entry
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &entries)
	}

	entries = append(entries, entry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Results:   results,
	})

	out, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0644)
}
