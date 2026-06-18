package commands

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"

	"agentsynch/internal/store"
)

func Claim() {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	agentID := fmt.Sprintf("agent-%s-%d", hostname, os.Getpid())

	db, err := store.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// claim next task atomically — worker mode if available, validator mode otherwise
	task, validatorMode, err := store.Claim(db, agentID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error claiming task: %v\n", err)
		os.Exit(1)
	}

	if task == nil {
		fmt.Println("no available tasks")
		return
	}

	if validatorMode {
		fmt.Printf("claimed task-%d for validation: %s (agent: %s)\n", task.ID, task.Title, agentID)
	} else {
		fmt.Printf("claimed task-%d: %s (agent: %s)\n", task.ID, task.Title, agentID)
		// print branch hint so the agent knows what to do
		if task.SameBranch {
			fmt.Println("hint: same-branch task — work on current branch, no new branch needed")
		} else {
			slug := titleSlug(task.Title)
			branchName := fmt.Sprintf("task-%d/%s", task.ID, slug)
			fmt.Printf("hint: create branch %s and record with set-branch --id %d --name %s\n", branchName, task.ID, branchName)
		}
	}

	// spawn a detached background heartbeat process so the task is not reaped as a zombie.
	// The `heartbeat` command loops internally (5-min intervals), so no shell wrapper is needed.
	// Setsid puts the subprocess in its own session so parent signals don't reach it.
	// Process.Release() frees Go's internal handle without killing the process; init will
	// reap it when it eventually exits (once the task leaves claimed/validating status).
	binary := os.Args[0]
	hb := exec.Command(binary, "heartbeat", "--id", fmt.Sprintf("%d", task.ID))
	hb.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := hb.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not start heartbeat: %v\n", err)
	} else {
		hb.Process.Release()
	}
}

var nonAlphanumDash = regexp.MustCompile(`[^a-z0-9-]+`)

// titleSlug converts a task title to a lowercase hyphenated slug for branch names.
func titleSlug(title string) string {
	s := strings.ToLower(title)
	s = strings.ReplaceAll(s, " ", "-")
	s = nonAlphanumDash.ReplaceAllString(s, "")
	// collapse consecutive hyphens
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	return s
}
