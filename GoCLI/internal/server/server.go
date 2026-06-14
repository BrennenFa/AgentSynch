package server

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	fmt.Println("agentsynch server started")

	reap(db)


	ticker := time.NewTicker(reapInterval)

	// create a channel to recieve os signals
	quit := make(chan os.Signal, 1)
	// signal quit when a SIGINT or SIGTERM is recieved
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		// ticker channel --> wait for a value to be sent
		case <-ticker.C:
			reap(db)

		// ensure that db connection closes --> not essential
		case  <-quit:
			fmt.Printf("server stopped")
			return
		}
	}
}
