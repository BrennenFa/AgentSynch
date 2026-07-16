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

// zombieAgent holds the pid/hostname info for a zombie task so we can attempt to kill it.
type zombieAgent struct {
	taskID   int64
	hostname string
	pid      int
}

// ReapZombies reclaims stale claimed tasks (zombie agents). Before resetting the DB,
// it attempts to kill the zombie process if it is running on the current machine.
// Tasks under maxAttempts are reset to 'available'; exhausted tasks are marked 'error'.
// Returns total rows affected.
func ReapZombies(db *sql.DB, timeout time.Duration) (int64, error) {
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
		SELECT id, COALESCE(agent_hostname, ''), COALESCE(agent_pid, 0)
		FROM tasks
		WHERE `+zombieWhere,
		threshold, threshold,
	)
	if err != nil {
		return 0, err
	}
	var zombies []zombieAgent
	for rows.Next() {
		var z zombieAgent
		if err := rows.Scan(&z.taskID, &z.hostname, &z.pid); err != nil {
			rows.Close()
			return 0, err
		}
		zombies = append(zombies, z)
	}
	rows.Close()

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
	r1, err := db.Exec(`
		UPDATE tasks
		SET status = 'available', claimed_by = NULL, claimed_at = NULL, heartbeat_at = NULL,
		    agent_hostname = NULL, agent_pid = NULL
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
		    agent_hostname = NULL, agent_pid = NULL,
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
