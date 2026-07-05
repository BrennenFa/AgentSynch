package store

import (
	"database/sql"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// newTestDB creates an isolated in-memory SQLite DB with the full schema.
// Each test gets its own named in-memory DB so they don't share state.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()

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

	initSchema := `
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

	if _, err := db.Exec(initSchema); err != nil {
		t.Fatalf("schema: %v", err)
	}

	t.Cleanup(func() { db.Close() })
	return db
}

// insertTask inserts a single task with the given status and returns its ID.
func insertTask(t *testing.T, db *sql.DB, title, status string) int64 {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := db.Exec(
		`INSERT INTO tasks (title, description, status, created_at) VALUES (?, 'test', ?, ?)`,
		title, status, now,
	)
	if err != nil {
		t.Fatalf("insertTask %q: %v", title, err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId: %v", err)
	}
	return id
}

// TestClaimAtomic launches N goroutines all calling Claim() simultaneously
// against a single available task and asserts exactly one succeeds.
func TestClaimAtomic(t *testing.T) {
	db := newTestDB(t)
	insertTask(t, db, "race-task", "available")

	const concurrency = 20
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		winners int
	)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			task, _, err := Claim(db, fmt.Sprintf("agent-%d", id))
			if err != nil {
				// Serialization failures expected under contention; not a double-claim.
				return
			}
			if task != nil {
				mu.Lock()
				winners++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	if winners != 1 {
		t.Errorf("expected exactly 1 winner, got %d", winners)
	}
}

// TestDependencyUnblocking inserts a blocked task that depends on a parent,
// finishes and validates the parent, and asserts the blocked task becomes available.
func TestDependencyUnblocking(t *testing.T) {
	db := newTestDB(t)

	// Insert parent as 'available'.
	parentID := insertTask(t, db, "parent-task", "available")

	// Insert child as 'blocked' with a dependency on parentID.
	now := time.Now().UTC().Format(time.RFC3339)
	childResult, err := db.Exec(
		`INSERT INTO tasks (title, description, status, created_at) VALUES ('child-task', 'test', 'blocked', ?)`,
		now,
	)
	if err != nil {
		t.Fatalf("insert child task: %v", err)
	}
	childID, _ := childResult.LastInsertId()

	_, err = db.Exec(
		`INSERT INTO task_dependencies (task_id, depends_on_id) VALUES (?, ?)`,
		childID, parentID,
	)
	if err != nil {
		t.Fatalf("insert dependency: %v", err)
	}

	// Claim the parent so we can finish it.
	task, _, err := Claim(db, "agent-parent")
	if err != nil {
		t.Fatalf("claim parent: %v", err)
	}
	if task == nil || task.ID != parentID {
		t.Fatalf("expected to claim parent (id=%d), got %v", parentID, task)
	}

	// Finish the parent (transitions to 'validating').
	if err := FinishTask(db, parentID, "done"); err != nil {
		t.Fatalf("finish parent: %v", err)
	}

	// Child should still be 'blocked' until the parent is validated (finished).
	var childStatus string
	if err := db.QueryRow(`SELECT status FROM tasks WHERE id = ?`, childID).Scan(&childStatus); err != nil {
		t.Fatalf("query child status: %v", err)
	}
	if childStatus != "blocked" {
		t.Errorf("expected child still blocked after parent enters validating, got %q", childStatus)
	}

	// Validate (approve) the parent — this should unblock the child.
	if err := ValidateTask(db, parentID, true, ""); err != nil {
		t.Fatalf("validate parent: %v", err)
	}

	// Now the child should be 'available'.
	if err := db.QueryRow(`SELECT status FROM tasks WHERE id = ?`, childID).Scan(&childStatus); err != nil {
		t.Fatalf("query child status after validate: %v", err)
	}
	if childStatus != "available" {
		t.Errorf("expected child to become available after parent validated, got %q", childStatus)
	}
}

// TestReaperRetryLogic inserts a stale claimed task (attempts < maxAttempts)
// and asserts ReapZombies() resets it to 'available'.
func TestReaperRetryLogic(t *testing.T) {
	db := newTestDB(t)

	// Insert a task claimed 2 hours ago with 1 attempt (under the 3-attempt cap).
	staleTime := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339)
	result, err := db.Exec(
		`INSERT INTO tasks (title, description, status, claimed_by, claimed_at, created_at, attempts)
		 VALUES ('stale-task', 'test', 'claimed', 'dead-agent', ?, ?, 1)`,
		staleTime, staleTime,
	)
	if err != nil {
		t.Fatalf("insert stale task: %v", err)
	}
	taskID, _ := result.LastInsertId()

	// Reap with a 1-minute timeout — stale for 2 hours, so it qualifies.
	n, err := ReapZombies(db, time.Minute)
	if err != nil {
		t.Fatalf("ReapZombies: %v", err)
	}
	if n == 0 {
		t.Fatal("expected ReapZombies to reclaim at least 1 task, got 0")
	}

	// The task should now be 'available' with cleared claim fields.
	var status string
	var claimedBy *string
	if err := db.QueryRow(`SELECT status, claimed_by FROM tasks WHERE id = ?`, taskID).Scan(&status, &claimedBy); err != nil {
		t.Fatalf("query task after reap: %v", err)
	}
	if status != "available" {
		t.Errorf("expected status 'available' after reap, got %q", status)
	}
	if claimedBy != nil {
		t.Errorf("expected claimed_by NULL after reap, got %q", *claimedBy)
	}
}

// TestMaxAttemptsError verifies that a zombie task with attempts >= maxAttempts
// transitions to 'error' rather than back to 'available'.
func TestMaxAttemptsError(t *testing.T) {
	db := newTestDB(t)

	// maxAttempts is 3 (defined in ReapZombies). Insert with attempts = 3.
	staleTime := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339)
	result, err := db.Exec(
		`INSERT INTO tasks (title, description, status, claimed_by, claimed_at, created_at, attempts)
		 VALUES ('exhausted-task', 'test', 'claimed', 'dead-agent', ?, ?, 3)`,
		staleTime, staleTime,
	)
	if err != nil {
		t.Fatalf("insert exhausted task: %v", err)
	}
	taskID, _ := result.LastInsertId()

	n, err := ReapZombies(db, time.Minute)
	if err != nil {
		t.Fatalf("ReapZombies: %v", err)
	}
	if n == 0 {
		t.Fatal("expected ReapZombies to affect at least 1 task, got 0")
	}

	var status string
	var taskErr *string
	if err := db.QueryRow(`SELECT status, error FROM tasks WHERE id = ?`, taskID).Scan(&status, &taskErr); err != nil {
		t.Fatalf("query task after reap: %v", err)
	}
	if status != "error" {
		t.Errorf("expected status 'error' after max attempts exhausted, got %q", status)
	}
	if taskErr == nil || *taskErr == "" {
		t.Error("expected non-empty error message after max attempts")
	}
}

// newBenchDB creates an isolated in-memory SQLite DB with the full schema.
func newBenchDB(b *testing.B) *sql.DB {
	b.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", b.Name())
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		b.Fatalf("open bench db: %v", err)
	}

	_, _ = db.Exec(`PRAGMA journal_mode=WAL;`)
	_, _ = db.Exec(`PRAGMA foreign_keys = ON;`)

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
		b.Fatalf("schema: %v", err)
	}
	b.Cleanup(func() { db.Close() })
	return db
}

// seedBenchTasks inserts n available tasks into the DB.
func seedBenchTasks(b *testing.B, db *sql.DB, n int) {
	b.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	for i := 0; i < n; i++ {
		if _, err := db.Exec(
			`INSERT INTO tasks (title, description, status, created_at) VALUES (?, ?, 'available', ?)`,
			fmt.Sprintf("bench-task-%d", i), "bench", now,
		); err != nil {
			b.Fatalf("seed task %d: %v", i, err)
		}
	}
}

// BenchmarkClaim measures concurrent claim throughput under contention.
// Sub-benchmarks vary the goroutine count: 2, 4, 8, 16, and GOMAXPROCS.
// Each sub-benchmark pre-populates a DB with b.N tasks, launches the goroutines,
// then measures total ops/sec.
func BenchmarkClaim(b *testing.B) {
	goroutineCounts := []int{2, 4, 8, 16, runtime.GOMAXPROCS(0)}

	// Deduplicate GOMAXPROCS if it matches one of the fixed counts.
	seen := make(map[int]bool)
	var counts []int
	for _, n := range goroutineCounts {
		if !seen[n] {
			seen[n] = true
			counts = append(counts, n)
		}
	}

	for _, n := range counts {
		n := n // capture
		b.Run(fmt.Sprintf("goroutines=%d", n), func(b *testing.B) {
			db := newBenchDB(b)

			// Seed enough tasks that all goroutines have work throughout the run.
			seedBenchTasks(b, db, b.N)

			var contention atomic.Int64

			b.ResetTimer()

			var wg sync.WaitGroup
			wg.Add(n)
			for i := 0; i < n; i++ {
				i := i
				go func() {
					defer wg.Done()
					agentID := fmt.Sprintf("bench-agent-%d", i)
					for {
						task, _, err := Claim(db, agentID)
						if err != nil {
							contention.Add(1)
							continue
						}
						if task == nil {
							return
						}
					}
				}()
			}
			wg.Wait()

			b.StopTimer()
			b.ReportMetric(float64(contention.Load()), "contention_errors")
		})
	}
}
