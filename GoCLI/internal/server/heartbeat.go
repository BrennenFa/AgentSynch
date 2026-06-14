package server

import (
	"flag"
	"fmt"
	"os"
	"time"

	"agentsynch/internal/store"
)

// Heartbeat is an internal command used to monitor if an agent exists
func Heartbeat() {
	flags := flag.NewFlagSet("heartbeat", flag.ExitOnError)

	// pass in task id
	idFlag := flags.Int64("id", 0, "task ID to heartbeat")
	flags.Parse(os.Args[2:])

	if *idFlag == 0 {
		fmt.Fprintln(os.Stderr, "No task id passed")
		os.Exit(1)
	}

	// open db to update
	db, err := store.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	for {
		time.Sleep(5 * time.Minute)

		// get and validate task
		task, err := store.GetTask(db, *idFlag)
		if err != nil || task == nil || task.Status != "claimed" {
			// task is done or gone — stop heartbeating
			return
		}

		if err := store.HeartbeatTask(db, *idFlag); err != nil {
			fmt.Fprintf(os.Stderr, "heartbeat error: %v\n", err)
		}
	}
}
