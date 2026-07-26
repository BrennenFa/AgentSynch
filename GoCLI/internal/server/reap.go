package server

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

// reapResult describes what happened to a single reaped task.
type reapResult struct {
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

func reap(db *sql.DB, ch chan string) {
	results, err := reapZombies(db, zombieTimeout)
	if err != nil {
		ch <- fmt.Sprintf("[%s] reaper error: %v", time.Now().Format("2006-01-02 15:04:05"), err)
		return
	}

	if len(results) == 0 {
		ch <- fmt.Sprintf("[%s] 0 tasks reaped", time.Now().Format("2006-01-02 15:04:05"))
		return
	}

	parts := make([]string, len(results))
	for i, r := range results {
		parts[i] = fmt.Sprintf("task-%d \"%s\" → %s", r.TaskID, r.Title, r.NewStatus)
	}
	ch <- fmt.Sprintf("[%s] %d tasks reaped - %s",
		time.Now().Format("2006-01-02 15:04:05"),
		len(results),
		strings.Join(parts, ", "),
	)
}

// reapZombies reclaims stale claimed tasks (zombie agents). Before resetting the DB,
// it attempts to kill the zombie process if it is running on the current machine.
// Tasks under maxAttempts are reset to 'available'; exhausted tasks are marked 'error'.
// Returns details of every reaped task.
func reapZombies(db *sql.DB, timeout time.Duration) ([]reapResult, error) {
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
	var results []reapResult
	for _, z := range zombies {
		status := "available"
		if z.attempts >= maxAttempts {
			status = "error"
		}
		results = append(results, reapResult{TaskID: z.taskID, Title: z.title, NewStatus: status})
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

	parts := make([]string, len(results))
	for i, r := range results {
		parts[i] = fmt.Sprintf("task-%d \"%s\" → %s", r.TaskID, r.Title, r.NewStatus)
	}
	ch <- fmt.Sprintf("[%s] %d tasks reaped - %s",
		time.Now().Format("2006-01-02 15:04:05"),
		len(results),
		strings.Join(parts, ", "),
	)
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
