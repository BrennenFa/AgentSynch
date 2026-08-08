package server

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"agentsynch/internal/config"
	"agentsynch/internal/server/tui"
	"agentsynch/internal/store"
)

const reapInterval = 30 * time.Second
const zombieTimeout = 1 * time.Minute

func TUI() {
	if _, err := loadOrSetupConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "error resolving config: %v\n", err)
		os.Exit(1)
	}

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

// loadOrSetupConfig loads the existing config, or — on first run (no config
// file yet) — prompts the user for a DB path and vault path and saves the result.
func loadOrSetupConfig() (config.Config, error) {
	exists, err := config.Exists()
	if err != nil {
		return config.Config{}, err
	}
	if exists {
		return config.Load()
	}

	fmt.Println("No AgentSynch config found — let's set one up.")
	fmt.Println()
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Task database path [default: ~/.agentsynch/tasks.db]:")
	fmt.Println("  Press Enter for default, or type a custom path:")
	fmt.Print("> ")
	dbPath, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return config.Config{}, err
	}
	dbPath = strings.TrimSpace(dbPath)

	fmt.Println()
	fmt.Println("Obsidian vault path (optional — enables task notes synced to a vault):")
	fmt.Println("  Press Enter to skip, or type your vault path:")
	fmt.Print("> ")
	vaultPath, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return config.Config{}, err
	}
	vaultPath = strings.TrimSpace(vaultPath)

	cfg := config.Config{DBPath: dbPath, VaultPath: vaultPath}
	if err := config.Save(cfg); err != nil {
		return config.Config{}, err
	}
	return cfg, nil
}
