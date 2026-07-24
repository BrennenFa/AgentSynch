package server

import (
	"fmt"
	"os"
	"time"

	"agentsynch/internal/server/tui"
	"agentsynch/internal/store"
)

const reapInterval = 30 * time.Second
const zombieTimeout = 1 * time.Minute

func TUI() {
	db, err := store.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	reapCh := make(chan string, 8)

	// run the reaper in the background — dies automatically when the process exits
	go func() {
		reap(db, reapCh)
		ticker := time.NewTicker(reapInterval)
		defer ticker.Stop()
		for range ticker.C {
			reap(db, reapCh)
		}
	}()

	// run the GitHub automation worker: creates PRs for finished tasks and Issues for error tasks
	go githubWorker(db)

	// run the Go TUI — blocks until the user quits
	tui.Run(db, reapCh)
}

