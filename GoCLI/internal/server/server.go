package server

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
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

	// find ui.py relative to this binary
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not locate binary: %v\n", err)
		os.Exit(1)
	}
	uiPath := filepath.Join(filepath.Dir(exe), "internal", "server", "ui", "ui.py")

	if _, err := os.Stat(uiPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "ui.py not found at %s\n", uiPath)
		fmt.Fprintf(os.Stderr, "install rich: pip install rich\n")
		os.Exit(1)
	}

	// spawn the Python TUI as a foreground process inheriting the terminal
	ui := exec.Command("python3", uiPath)
	ui.Stdin  = os.Stdin
	ui.Stdout = os.Stdout
	ui.Stderr = os.Stderr

	// forward SIGINT/SIGTERM to the UI process so it can clean up the terminal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		s := <-sig
		if ui.Process != nil {
			ui.Process.Signal(s)
		}
	}()

	// block until the UI exits — then the whole process exits, killing the reaper goroutine
	ui.Run()
}
