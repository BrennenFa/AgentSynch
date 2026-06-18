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


			if err := checkoutNewBranch(branchName); err != nil {
				fmt.Printf("warning: could not create branch %s: %v\n", branchName, err)
				fmt.Printf("hint: create branch %s and record with set-branch --id %d --name %s\n", branchName, task.ID, branchName)
			} else {
				if err := store.SetBranchName(db, task.ID, branchName); err != nil {
					fmt.Printf("warning: could not record branch name: %v\n", err)
				}
				fmt.Printf("hint: created branch %s\n", branchName)
			}
		}
	}
	// print title as its own output field so it is unambiguous regardless of claim format
	fmt.Printf("title: %s\n", task.Title)

	// spawn a detached background heartbeat loop so the task is not reaped as a zombie;
	// uses the same binary that is currently running so no extra setup is needed
	binary := os.Args[0]

	// run a subprocess that heartbeats at a predefined interval
	script := fmt.Sprintf("while true; do sleep 600; %s heartbeat --id %d; done", binary, task.ID)
	hb := exec.Command("sh", "-c", script)
	hb.Start() // detach — we never call Wait(); the shell loop outlives this process
}

