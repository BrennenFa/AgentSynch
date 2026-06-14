package server

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	"agentsynch/internal/store"
)

func reap(db *sql.DB) {

	// call a function to monitor the db
	n, err := store.ReapZombies(db, zombieTimeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[reap] error: %v\n", err)
		return
	}

	// print if zombies detected
	if n > 0 {
		fmt.Printf("[reap] %s — reclaimed %d zombie task(s)\n", time.Now().Format(time.RFC3339), n)
	}
}
