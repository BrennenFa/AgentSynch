package tests

import (
	"fmt"
	"testing"
	"time"

	"agentsynch/internal/store"
)

// TestDependencyUnblocking verifies that when a blocking task is finished and
// validated, its dependent task transitions from blocked to available.
func TestDependencyUnblocking(t *testing.T) {
	db := newTestDB(t)

	now := time.Now().UTC().Format(time.RFC3339)

	// Insert parent task as available
	res, err := db.Exec(
		`INSERT INTO tasks (title, description, status, created_at) VALUES (?, ?, 'available', ?)`,
		"parent-task", "the blocking task", now,
	)
	if err != nil {
		t.Fatalf("insert parent: %v", err)
	}
	parentID, _ := res.LastInsertId()

	// Insert child task as blocked
	res, err = db.Exec(
		`INSERT INTO tasks (title, description, status, created_at) VALUES (?, ?, 'blocked', ?)`,
		"child-task", "depends on parent", now,
	)
	if err != nil {
		t.Fatalf("insert child: %v", err)
	}
	childID, _ := res.LastInsertId()

	// Record dependency: child depends on parent
	_, err = db.Exec(
		`INSERT INTO task_dependencies (task_id, depends_on_id) VALUES (?, ?)`,
		childID, parentID,
	)
	if err != nil {
		t.Fatalf("insert dependency: %v", err)
	}

	// Claim and finish the parent task
	task, isVal, err := store.Claim(db, "agent-worker")
	if err != nil {
		t.Fatalf("claim error: %v", err)
	}
	if task == nil {
		t.Fatal("expected to claim parent task, got nil")
	}
	if isVal {
		t.Fatal("expected worker claim, got validator mode")
	}
	if task.ID != parentID {
		t.Fatalf("expected to claim parent (id=%d), got id=%d", parentID, task.ID)
	}

	// Move parent to validating
	if err := store.FinishTask(db, parentID, "done"); err != nil {
		t.Fatalf("finish parent: %v", err)
	}

	// Child should still be blocked while parent is validating
	var childStatus string
	if err := db.QueryRow(`SELECT status FROM tasks WHERE id = ?`, childID).Scan(&childStatus); err != nil {
		t.Fatalf("query child status: %v", err)
	}
	if childStatus != "blocked" {
		t.Errorf("expected child to still be blocked while parent is validating, got %s", childStatus)
	}

	// Validate (approve) the parent — this should unblock the child
	if err := store.ValidateTask(db, parentID, true, ""); err != nil {
		t.Fatalf("validate parent: %v", err)
	}

	// Child should now be available
	if err := db.QueryRow(`SELECT status FROM tasks WHERE id = ?`, childID).Scan(&childStatus); err != nil {
		t.Fatalf("query child status after validation: %v", err)
	}
	if childStatus != "available" {
		t.Errorf("expected child to be available after parent finished, got %s", childStatus)
	}
}

// TestZombieReaping verifies that a stale claimed task (no heartbeat within the
// timeout window, attempts below the max) is reset to available by ReapZombies.
func TestZombieReaping(t *testing.T) {
	db := newTestDB(t)

	// Insert a task that was claimed 10 minutes ago with no heartbeat
	staleTime := time.Now().UTC().Add(-10 * time.Minute).Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO tasks (title, description, status, claimed_by, claimed_at, created_at, attempts)
		 VALUES (?, ?, 'claimed', ?, ?, ?, ?)`,
		"stale-task", "zombie agent dropped this", "dead-agent", staleTime, staleTime, 1,
	)
	if err != nil {
		t.Fatalf("insert stale task: %v", err)
	}

	// Reap with a 5-minute timeout — our task is 10 min old, so it qualifies
	n, err := store.ReapZombies(db, 5*time.Minute)
	if err != nil {
		t.Fatalf("ReapZombies error: %v", err)
	}
	if n == 0 {
		t.Error("expected ReapZombies to affect at least 1 row, got 0")
	}

	// Task should now be available again
	var status string
	if err := db.QueryRow(`SELECT status FROM tasks WHERE title = 'stale-task'`).Scan(&status); err != nil {
		t.Fatalf("query status: %v", err)
	}
	if status != "available" {
		t.Errorf("expected zombie task to be reset to available, got %s", status)
	}
}

// TestMaxAttemptsError verifies that a stale claimed task whose attempt count has
// reached or exceeded the max (3) is moved to error rather than retried.
func TestMaxAttemptsError(t *testing.T) {
	db := newTestDB(t)

	staleTime := time.Now().UTC().Add(-10 * time.Minute).Format(time.RFC3339)

	// Insert tasks at each attempt count to confirm the boundary
	for _, attempts := range []int{3, 4, 5} {
		_, err := db.Exec(
			`INSERT INTO tasks (title, description, status, claimed_by, claimed_at, created_at, attempts)
			 VALUES (?, ?, 'claimed', ?, ?, ?, ?)`,
			fmt.Sprintf("exhausted-task-%d", attempts), "max retries hit", "dead-agent", staleTime, staleTime, attempts,
		)
		if err != nil {
			t.Fatalf("insert task (attempts=%d): %v", attempts, err)
		}
	}

	n, err := store.ReapZombies(db, 5*time.Minute)
	if err != nil {
		t.Fatalf("ReapZombies error: %v", err)
	}
	if n == 0 {
		t.Error("expected ReapZombies to affect rows, got 0")
	}

	// All three tasks should be in error status now
	rows, err := db.Query(`SELECT title, status, error FROM tasks WHERE title LIKE 'exhausted-task-%'`)
	if err != nil {
		t.Fatalf("query exhausted tasks: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var title, status string
		var errMsg *string
		if err := rows.Scan(&title, &status, &errMsg); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if status != "error" {
			t.Errorf("%s: expected status=error, got %s", title, status)
		}
		if errMsg == nil || *errMsg == "" {
			t.Errorf("%s: expected non-empty error message", title)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows error: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 exhausted tasks, found %d", count)
	}
}
