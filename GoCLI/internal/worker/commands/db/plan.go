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

	cfg, err := config.Load()
	if err != nil || cfg.VaultPath == "" {
		fmt.Fprintln(os.Stderr, "error: vault is not configured — vault is the source of truth for plans")
		os.Exit(1)
	}

	db, err := store.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// verify task exists and is claimed before writing plan
	task, err := store.GetTask(db, *idFlag)
	if err != nil || task == nil {
		fmt.Fprintf(os.Stderr, "error: task-%d not found\n", *idFlag)
		os.Exit(1)
	}
	if task.Status != "claimed" {
		fmt.Fprintf(os.Stderr, "error: task-%d is not claimed (status: %s)\n", *idFlag, task.Status)
		os.Exit(1)
	}

	agentID := ""
	if task.ClaimedBy != nil {
		agentID = *task.ClaimedBy
	}
	repoName := vault.RepoName()
	if err := vault.CreateTaskNote(cfg.VaultPath, repoName, *idFlag, task.Title, task.Description, task.Status, agentID); err != nil {
		fmt.Fprintf(os.Stderr, "error: vault: could not create task note: %v\n", err)
		os.Exit(1)
	}
	if err := vault.WritePlan(cfg.VaultPath, repoName, *idFlag, *planFlag); err != nil {
		fmt.Fprintf(os.Stderr, "error: vault: could not write plan: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("plan written for task-%d\n", *idFlag)
}
