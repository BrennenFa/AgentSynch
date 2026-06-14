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

	// claim next avaiable task
	task, err := store.ClaimTask(db, agentID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error claiming task: %v\n", err)
		os.Exit(1)
	}

	if task == nil {
		fmt.Println("no available tasks")
		return
	}

	fmt.Printf("claimed task-%d: %s (agent: %s)\n", task.ID, task.Title, agentID)

	// spawn a detached background heartbeat loop so the task is not reaped as a zombie;
	// uses the same binary that is currently running so no extra setup is needed
	binary := os.Args[0]
	script := fmt.Sprintf("while true; do sleep 600; %s heartbeat --id %d; done", binary, task.ID)
	hb := exec.Command("sh", "-c", script)
	hb.Start() // detach — we never call Wait(); the shell loop outlives this process
}
