package commands

import (
	"fmt"
	"os"
	"time"

	"agentsynch/store"
)

func Claim() {
	agentID := fmt.Sprintf("agent-%d", time.Now().Unix())

	db, err := store.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

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
}
