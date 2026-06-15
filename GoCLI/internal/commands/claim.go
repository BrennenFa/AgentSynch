package commands

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

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

	// spawn a detached background heartbeat loop so the task is not reaped as a zombie;
	// uses the same binary that is currently running so no extra setup is needed
	binary := os.Args[0]

	// run a subprocess that heartbeats at a predefined interval
	script := fmt.Sprintf("while true; do sleep 600; %s heartbeat --id %d; done", binary, task.ID)
	hb := exec.Command("sh", "-c", script)
	hb.Start() // detach — we never call Wait(); the shell loop outlives this process
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
