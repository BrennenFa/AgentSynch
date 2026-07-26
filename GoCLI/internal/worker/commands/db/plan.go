package db

import (
	"flag"
	"fmt"
	"os"

	"agentsynch/internal/config"
	"agentsynch/internal/store"
	"agentsynch/internal/vault"
)

func Plan() {
	flags := flag.NewFlagSet("plan", flag.ExitOnError)
	idFlag := flags.Int64("id", 0, "task id")
	planFlag := flags.String("plan", "", "plan for the task")
	flags.Parse(os.Args[2:])

	if *idFlag == 0 || *planFlag == "" {
		fmt.Fprintln(os.Stderr, "usage: plan --id <id> --plan <plan>")
		os.Exit(1)
	}

	db, err := store.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := store.WritePlan(db, *idFlag, *planFlag); err != nil {
		fmt.Fprintf(os.Stderr, "error writing plan: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("plan written for task-%d\n", *idFlag)

	// write plan to Obsidian vault if configured
	cfg, err := config.Load()
	if err != nil || cfg.VaultPath == "" {
		return
	}
	task, err := store.GetTask(db, *idFlag)
	if err != nil || task == nil {
		fmt.Fprintf(os.Stderr, "warning: notes: could not fetch task: %v\n", err)
		return
	}
	agentID := ""
	if task.ClaimedBy != nil {
		agentID = *task.ClaimedBy
	}
	repoName := vault.RepoName()
	if err := vault.CreateTaskNote(cfg.VaultPath, repoName, *idFlag, task.Title, task.Description, task.Status, agentID); err != nil {
		fmt.Fprintf(os.Stderr, "warning: vault: could not create task note: %v\n", err)
	}
	if err := vault.WritePlan(cfg.VaultPath, repoName, *idFlag, *planFlag); err != nil {
		fmt.Fprintf(os.Stderr, "warning: vault: could not write plan: %v\n", err)
	}
}
