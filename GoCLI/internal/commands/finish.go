package commands

import (
	"flag"
	"fmt"
	"os"

	"agentsynch/internal/store"
)

func Finish() {
	flags := flag.NewFlagSet("finish", flag.ExitOnError)
	idFlag := flags.Int64("id", 0, "task id")
	outputFlag := flags.String("output", "", "summary of what was done")
	errorFlag := flags.String("error", "", "error message if task failed")
	flags.Parse(os.Args[2:])

	if *idFlag == 0 {
		fmt.Fprintln(os.Stderr, "usage: finish --id <id> --output <summary>")
		fmt.Fprintln(os.Stderr, "       finish --id <id> --error <message>")
		os.Exit(1)
	}

	db, err := store.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Auto-push branch if one was recorded for this task
	// TODO --- what does recorderd for this task mean??? is it useful?
	task, err := store.GetTask(db, *idFlag)
	
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not fetch task for branch push: %v\n", err)
	} else if task != nil && task.BranchName != nil && *task.BranchName != "" {
		// task is available and has a branch name recorded. --> try to push it
		if err := pushBranch(*task.BranchName); err != nil {
			fmt.Printf("warning: could not push branch %s: %v\n", *task.BranchName, err)
		} else {
			fmt.Printf("pushed branch %s to origin\n", *task.BranchName)
		}
	}

	// Case 1: an error exists
	if *errorFlag != "" {
		if err := store.ErrorTask(db, *idFlag, *errorFlag); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("task-%d marked as error\n", *idFlag)
		return
	}


	// Default Case --> task marked finished
	if err := store.FinishTask(db, *idFlag, *outputFlag); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("task-%d marked as finished\n", *idFlag)
}
