package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

-- DAG task dependency table
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

	// add new columns for validator tracking; ignore "duplicate column name" on existing DBs
	migrations := []string{
		`ALTER TABLE tasks ADD COLUMN validator_id TEXT`,
		`ALTER TABLE tasks ADD COLUMN validation_claimed_at TEXT`,
		// branch_name: the branch the agent created and worked on (NULL if not yet set)
		`ALTER TABLE tasks ADD COLUMN branch_name TEXT`,
		// gh_url: URL of the GitHub PR or issue created by the server; guards against duplicate GH actions
		`ALTER TABLE tasks ADD COLUMN gh_url TEXT`,
	}
	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				db.Close()
				return nil, fmt.Errorf("migration failed: %w", err)
			}
		}
	}

	return db, nil
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
