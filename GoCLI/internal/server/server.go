package server

import (
	"fmt"
	"os"
	"time"

	"agentsynch/internal/server/tui"
	"agentsynch/internal/store"
)

const reapInterval = 5 * time.Minute
const zombieTimeout = 10 * time.Minute

func Server() {
	db, err := store.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// run the reaper in the background — dies automatically when the process exits
	go func() {
		reap(db)
		ticker := time.NewTicker(reapInterval)
		defer ticker.Stop()
		for range ticker.C {
			reap(db)
		}
	}()

	// run the GitHub automation worker: creates PRs for finished tasks and Issues for error tasks
	go githubWorker(db)

	// run the Go TUI — blocks until the user quits
	tui.Run(db)
}

// TUI opens the database and runs the standalone TUI dashboard.
func TUI() {
	db, err := store.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	tui.Run(db)
}
