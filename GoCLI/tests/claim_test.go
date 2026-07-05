package tests

import (
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"agentsynch/internal/store"
	_ "modernc.org/sqlite"
)

// TestNoDoubleClaim verifies that exactly one goroutine wins when 20 agents
// race to claim the single available task.
func TestNoDoubleClaim(t *testing.T) {
	db := newTestDB(t)
	seedTasks(t, db, 1, "available")

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		winners int
	)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			task, err := store.Claim(db, fmt.Sprintf("agent-%d", id))
			if err != nil {
				return // serialization errors expected under contention; not a double-claim
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

// TestEmptyDBReturnsNil verifies that Claim on an empty DB returns (nil, nil).
func TestEmptyDBReturnsNil(t *testing.T) {
	db := newTestDB(t)

	task, err := store.Claim(db, "agent-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task != nil {
		t.Errorf("expected nil task, got %+v", task)
	}
}

// TestScalingMetrics runs a claim+finish simulation with 1, 2, 4, 8, 16 concurrent
// agents and prints an ASCII table showing throughput scaling.
func TestScalingMetrics(t *testing.T) {
	const (
		nTasks    = 100
		simTokens = 500   // tokens per task (simulated LLM context)
		simWorkUs = 10000 // microseconds of simulated work per task (~10ms)
	)

	agentCounts := []int{1, 2, 4, 8, 16}
	var allResults []MetricsResult

	baselineThroughput := 0.0

	fmt.Println()
	fmt.Println("=== AgentSynch Parallelization Metrics ===")
	fmt.Printf("%-8s %-12s %-16s %-12s %-12s\n",
		"Agents", "Tasks/sec", "Sim Tok/sec", "Contention", "Speedup vs 1")

	for _, n := range agentCounts {
		db := newTestDB(t)
		seedTasks(t, db, nTasks, "available")

		var (
			wg               sync.WaitGroup
			mu               sync.Mutex
			tasksCompleted   int
			contentionErrors atomic.Int64
		)

		start := time.Now()

		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(agentID string) {
				defer wg.Done()
				for {
					task, err := store.Claim(db, agentID)
					if err != nil {
						contentionErrors.Add(1)
						continue
					}
					if task == nil {
						return
					}

					// Simulate LLM work
					time.Sleep(time.Duration(simWorkUs) * time.Microsecond)

					_ = store.FinishTask(db, task.ID, "simulated output")

					mu.Lock()
					tasksCompleted++
					mu.Unlock()
				}
			}(fmt.Sprintf("agent-%d-%d", n, i))
		}

		wg.Wait()
		durationMs := time.Since(start).Milliseconds()

		durationSec := float64(durationMs) / 1000.0
		throughput := float64(tasksCompleted) / durationSec
		simTokPerSec := float64(tasksCompleted) * simTokens / durationSec

		if baselineThroughput == 0 && throughput > 0 {
			baselineThroughput = throughput
		}
		speedup := throughput / baselineThroughput

		fmt.Printf("%-8d %-12.1f %-16.0f %-12d %.1fx\n",
			n, throughput, simTokPerSec, contentionErrors.Load(), speedup)

		allResults = append(allResults, MetricsResult{
			AgentCount:       n,
			TotalTasks:       nTasks,
			TasksCompleted:   tasksCompleted,
			DurationMs:       durationMs,
			ThroughputPerSec: throughput,
			SimTokensPerSec:  simTokPerSec,
			ContentionErrors: contentionErrors.Load(),
		})
	}

	fmt.Println()

	if err := writeMetricsJSON(allResults); err != nil {
		t.Logf("warning: could not write metrics.json: %v", err)
	}

	// Sanity: 16-agent run should be meaningfully faster than 1-agent
	if len(allResults) >= 2 {
		first := allResults[0].ThroughputPerSec
		last := allResults[len(allResults)-1].ThroughputPerSec
		if last < first*1.5 {
			t.Errorf("expected 16-agent throughput (%.1f) >= 1.5x 1-agent throughput (%.1f)", last, first)
		}
	}
}

// BenchmarkClaim_Sequential seeds b.N tasks then claims them sequentially.
func BenchmarkClaim_Sequential(b *testing.B) {
	db := newBenchDB(b)
	defer db.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	for i := 0; i < b.N; i++ {
		if _, err := db.Exec(
			`INSERT INTO tasks (title, description, status, created_at) VALUES (?, ?, 'available', ?)`,
			fmt.Sprintf("bench-task-%d", i), "bench", now,
		); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		task, err := store.Claim(db, "bench-agent")
		if err != nil || task == nil {
			b.Fatalf("claim failed: err=%v task=%v", err, task)
		}
	}
}

// BenchmarkClaim_Concurrent seeds 10000 tasks and claims them with b.RunParallel.
func BenchmarkClaim_Concurrent(b *testing.B) {
	db := newBenchDB(b)
	defer db.Close()

	const totalTasks = 10000
	now := time.Now().UTC().Format(time.RFC3339)
	for i := 0; i < totalTasks; i++ {
		if _, err := db.Exec(
			`INSERT INTO tasks (title, description, status, created_at) VALUES (?, ?, 'available', ?)`,
			fmt.Sprintf("bench-task-%d", i), "bench", now,
		); err != nil {
			b.Fatal(err)
		}
	}

	var contention atomic.Int64

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		agentID := fmt.Sprintf("bench-agent-%p", pb)
		for pb.Next() {
			_, err := store.Claim(db, agentID)
			if err != nil {
				contention.Add(1)
			}
		}
	})
	b.StopTimer()

	b.Logf("contention errors: %d", contention.Load())
}

// newBenchDB creates an in-memory SQLite DB for benchmarks.
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
	return db
}
