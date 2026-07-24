package store

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
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

// ReapResult describes what happened to a single reaped task.
type ReapResult struct {
	TaskID    int64
	Title     string
	NewStatus string // "available" or "error"
}

// zombieAgent holds the pid/hostname/title info for a zombie task.
type zombieAgent struct {
	taskID   int64
	title    string
	hostname string
	pid      int
	attempts int
}

// ReapZombies reclaims stale claimed tasks (zombie agents). Before resetting the DB,
// it attempts to kill the zombie process if it is running on the current machine.
// Tasks under maxAttempts are reset to 'available'; exhausted tasks are marked 'error'.
// Returns details of every reaped task.
func ReapZombies(db *sql.DB, timeout time.Duration) ([]ReapResult, error) {
	const maxAttempts = 3

	threshold := time.Now().UTC().Add(-timeout).Format(time.RFC3339)
	zombieWhere := `
		status = 'claimed'
		AND (
		    (heartbeat_at IS NOT NULL AND heartbeat_at < ?)
		    OR (heartbeat_at IS NULL AND claimed_at < ?)
		)`

	// query zombie tasks first so we can attempt to kill their processes
	rows, err := db.Query(`
		SELECT id, title, COALESCE(agent_hostname, ''), COALESCE(agent_pid, 0), attempts
		FROM tasks
		WHERE `+zombieWhere,
		threshold, threshold,
	)
	if err != nil {
		return nil, err
	}
	var zombies []zombieAgent
	for rows.Next() {
		var z zombieAgent
		if err := rows.Scan(&z.taskID, &z.title, &z.hostname, &z.pid, &z.attempts); err != nil {
			rows.Close()
			return nil, err
		}
		zombies = append(zombies, z)
	}
	rows.Close()

	if len(zombies) == 0 {
		return nil, nil
	}

	// attempt to kill each zombie process if it is on this machine
	localHostname, _ := os.Hostname()
	for _, z := range zombies {
		if z.pid == 0 || z.hostname != localHostname {
			if z.hostname != localHostname && z.hostname != "" {
				fmt.Fprintf(os.Stderr, "[reap] task-%d: zombie on remote host %q — skipping kill\n", z.taskID, z.hostname)
			}
			continue
		}
		killZombieProcess(z.taskID, z.pid)
	}

	// tasks under the attempt limit go back to available for retry
	_, err = db.Exec(`
		UPDATE tasks
		SET status = 'available', claimed_by = NULL, claimed_at = NULL, heartbeat_at = NULL,
		    agent_hostname = NULL, agent_pid = NULL
		WHERE `+zombieWhere+` AND attempts < ?`,
		threshold, threshold, maxAttempts,
	)
	if err != nil {
		return nil, err
	}

	// tasks that have hit the limit are marked error (no more retries)
	_, err = db.Exec(`
		UPDATE tasks
		SET status = 'error', claimed_by = NULL, claimed_at = NULL, heartbeat_at = NULL,
		    agent_hostname = NULL, agent_pid = NULL,
		    error = 'max attempts reached: agent did not complete task'
		WHERE `+zombieWhere+` AND attempts >= ?`,
		threshold, threshold, maxAttempts,
	)
	if err != nil {
		return nil, err
	}

	// build results — determine new status from attempt count
	var results []ReapResult
	for _, z := range zombies {
		status := "available"
		if z.attempts >= maxAttempts {
			status = "error"
		}
		results = append(results, ReapResult{TaskID: z.taskID, Title: z.title, NewStatus: status})
	}
	return results, nil
}

// killZombieProcess sends SIGTERM to pid, waits briefly, then SIGKILLs if still alive.
// It verifies the process cmdline contains the binary name before killing to guard
// against PID reuse by an unrelated process.
func killZombieProcess(taskID int64, pid int) {
	if !pidBelongsToAgent(pid) {
		fmt.Fprintf(os.Stderr, "[reap] task-%d: pid %d exists but cmdline does not match agentsynch binary — skipping kill\n", taskID, pid)
		return
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}

	// SIGTERM first — give the process a chance to clean up
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return // process already gone
	}
	fmt.Fprintf(os.Stderr, "[reap] task-%d: sent SIGTERM to pid %d\n", taskID, pid)

	// wait up to 3 seconds, then SIGKILL
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			return // process exited cleanly
		}
	}

	if err := proc.Signal(syscall.SIGKILL); err == nil {
		fmt.Fprintf(os.Stderr, "[reap] task-%d: sent SIGKILL to pid %d\n", taskID, pid)
	}
}

// pidBelongsToAgent checks whether the given pid's command line contains the
// agentsynch binary name, to avoid killing an unrelated process that reused the pid.
func pidBelongsToAgent(pid int) bool {
	// use `ps` since /proc is not available on macOS
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=").Output()
	if err != nil {
		return false // process not found
	}
	cmdline := strings.ToLower(strings.TrimSpace(string(out)))
	return strings.Contains(cmdline, "agentsynch") || strings.Contains(cmdline, "go run")
}
