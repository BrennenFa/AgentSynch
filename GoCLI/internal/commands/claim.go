package commands

import (
	"fmt"
	"os"
	"os/exec"

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
	
	// looking at a validation task
	if validatorMode {
		fmt.Printf("claimed task-%d for validation: %s (agent: %s)\n", task.ID, task.Title, agentID)
		
	// looking at engineering task
	} else {
		fmt.Printf("claimed task-%d: %s (agent: %s)\n", task.ID, task.Title, agentID)

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
				if err := checkoutNewBranch(candidate); err == nil {
					created = candidate
					break
				}
			}
			if created == "" {
				fmt.Printf("warning: could not create branch %s (tried up to -10 suffix)\n", branchName)
				fmt.Printf("hint: create branch %s and record with set-branch --id %d --name %s\n", branchName, task.ID, branchName)
			} else {
				if err := store.SetBranchName(db, task.ID, created); err != nil {
					fmt.Printf("warning: could not record branch name: %v\n", err)
				}
				fmt.Printf("hint: created branch %s\n", created)
			}
		}
	}
	// print title as its own output field so it is unambiguous regardless of claim format
	fmt.Printf("title: %s\n", task.Title)

	// spawn a detached background heartbeat loop so the task is not reaped as a zombie;
	// re-invoke this same executable so no extra setup is needed
	selfExe := os.Args[0] // path to the currently-running executable (e.g. ./agentsynch)

	// run a subprocess that heartbeats at a predefined interval
	script := fmt.Sprintf("while true; do sleep 600; %s heartbeat --id %d; done", selfExe, task.ID)
	hb := exec.Command("sh", "-c", script)
	hb.Start() // detach — we never call Wait(); the shell loop outlives this process
}

