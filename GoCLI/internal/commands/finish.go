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

	// fetch task to get branch name and metadata for PR/issue creation
	task, err := store.GetTask(db, *idFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not fetch task: %v\n", err)
	}

	hasBranch := task != nil && task.BranchName != nil && *task.BranchName != ""

	// push branch if one was recorded for this task
	if hasBranch {
		if err := pushBranch(*task.BranchName); err != nil {
			fmt.Printf("warning: could not push branch %s: %v\n", *task.BranchName, err)
			hasBranch = false
		} else {
			fmt.Printf("pushed branch %s to origin\n", *task.BranchName)
		}
	}

	// Case 1: error — mark task, open GitHub issue
	if *errorFlag != "" {
		if err := store.ErrorTask(db, *idFlag, *errorFlag); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("task-%d marked as error\n", *idFlag)
		// re-fetch task so createIssue sees the true DB state after ErrorTask
		task, err = store.GetTask(db, *idFlag)
		if err != nil || task == nil {
			fmt.Printf("warning: could not re-fetch task for issue: %v\n", err)
		} else {
			if url, err := createIssue(*task); err != nil {
				fmt.Printf("warning: could not create GitHub issue: %v\n", err)
			} else {
				_ = store.SetDbGit(db, *idFlag, url)
				fmt.Printf("issue: %s\n", url)
			}
		}
		return
	}

	// Default case — mark finished, open GitHub PR
	if err := store.FinishTask(db, *idFlag, *outputFlag); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("task-%d marked as finished\n", *idFlag)

	if hasBranch {
		// re-fetch task so createPR sees the true DB state after FinishTask
		task, err = store.GetTask(db, *idFlag)
		if err != nil || task == nil {
			fmt.Printf("warning: could not re-fetch task for PR: %v\n", err)
		} else {
			if url, err := createPR(*task); err != nil {
				fmt.Printf("warning: could not create GitHub PR: %v\n", err)
			} else {
				_ = store.SetDbGit(db, *idFlag, url)
				fmt.Printf("pr: %s\n", url)
			}
		}
	}
}
