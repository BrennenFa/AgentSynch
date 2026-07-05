package tests

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

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
func seedTasks(t *testing.T, db *sql.DB, n int, status string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	for i := 0; i < n; i++ {
		_, err := db.Exec(
			`INSERT INTO tasks (title, description, status, created_at) VALUES (?, ?, ?, ?)`,
			fmt.Sprintf("task-%d", i), "test task", status, now,
		)
		if err != nil {
			t.Fatalf("seed task %d: %v", i, err)
		}
	}
}

// MetricsResult holds the output of one scaling test run.
type MetricsResult struct {
	AgentCount       int     `json:"agent_count"`
	TotalTasks       int     `json:"total_tasks"`
	TasksCompleted   int     `json:"tasks_completed"`
	DurationMs       int64   `json:"duration_ms"`
	ThroughputPerSec float64 `json:"throughput_per_sec"`
	SimTokensPerSec  float64 `json:"sim_tokens_per_sec"`
	ContentionErrors int64   `json:"contention_errors"`
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
