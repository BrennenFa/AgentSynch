package commands

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"agentsynch/internal/store"
)

func Claim() {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// unique agent id based on hostname and process id
	agentID := fmt.Sprintf("agent-%s-%d", hostname, os.Getpid())

	db, err := store.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// claim next available task atomically
	task, err := store.Claim(db, agentID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error claiming task: %v\n", err)
		os.Exit(1)
	}

	if task == nil {
		fmt.Println("no available tasks")
		return
	}

	fmt.Printf("claimed task-%d: %s (agent: %s)\n", task.ID, task.Title, agentID)
	fmt.Printf("title: %s \n", task.Title)
	// print branch hint so the agent knows what to do
	// comes from --same-branch flag
	if task.SameBranch {
		fmt.Println("hint: same-branch task — work on current branch, no new branch needed")
	} else {
		slug := titleSlug(task.Title)
		branchName := fmt.Sprintf("task-%d/%s", task.ID, slug)

		// retry with numeric suffix if branch already exists
		created := ""
		for attempt := 1; attempt <= 10; attempt++ {
			candidate := branchName
			if attempt > 1 {
				candidate = fmt.Sprintf("%s-%d", branchName, attempt)
			}
			if err := createWorktree("../AgentSynch-"+candidate, candidate); err == nil {
				created = candidate
				break
			}
		}
		if created == "" {
			fmt.Printf("warning: could not create branch %s (tried up to -10 suffix)\n", branchName)
		} else {
			if err := store.SetBranchName(db, task.ID, created); err != nil {
				fmt.Printf("warning: could not record branch name: %v\n", err)
			}
			fmt.Printf("hint: created branch %s in worktree ../AgentSynch-%s\n", created, created)
		}
	}
	// print title as its own output field so it is unambiguous regardless of claim format
	fmt.Printf("title: %s\n", task.Title)

	// heartbeat for reaper...
	binaryPath := os.Args[0]

	// build command for heartbeat process
	heartbeat := exec.Command(binaryPath, "heartbeat", "--id", fmt.Sprintf("%d", task.ID))

	// define heartbeat process to run in its own session
	heartbeat.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := heartbeat.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not start heartbeat: %v\n", err)
	} else {
		heartbeat.Process.Release()
	}
}
