package db

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"os/exec"

	"agentsynch/internal/config"
	"agentsynch/internal/store"
	"agentsynch/internal/vault"
	"agentsynch/internal/worker/commands/system"
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

	// auto-commit any uncommitted work before pushing
	if hasBranch && task != nil {
		autoCommit(task.ID, task.Title)
	}

	// push branch if one was recorded for this task
	if hasBranch {
		if err := system.PushBranch(*task.BranchName); err != nil {
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
		appendToNotes(db, *idFlag, *errorFlag, "error")
		// re-fetch task so CreateIssue sees the true DB state after ErrorTask
		task, err = store.GetTask(db, *idFlag)
		if err != nil || task == nil {
			fmt.Printf("warning: could not re-fetch task for issue: %v\n", err)
		} else {
			if url, err := system.CreateIssue(*task); err != nil {
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
	appendToNotes(db, *idFlag, *outputFlag, "finished")

	if hasBranch {
		// re-fetch task so CreatePR sees the true DB state after FinishTask
		task, err = store.GetTask(db, *idFlag)
		if err != nil || task == nil {
			fmt.Printf("warning: could not re-fetch task for PR: %v\n", err)
		} else {
			if url, err := system.CreatePR(*task); err != nil {
				fmt.Printf("warning: could not create GitHub PR: %v\n", err)
			} else {
				_ = store.SetDbGit(db, *idFlag, url)
				fmt.Printf("pr: %s\n", url)
			}
		}
	}
}

// autoCommit stages all changes and commits them. If there is nothing to commit, it is a no-op.
// Errors are warnings only — never blocks finish.
func autoCommit(taskID int64, title string) {
	if err := exec.Command("git", "add", "-A").Run(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: auto-commit: git add failed: %v\n", err)
		return
	}
	msg := fmt.Sprintf("task-%d: %s", taskID, title)
	out, err := exec.Command("git", "commit", "-m", msg).CombinedOutput()
	if err != nil {
		// "nothing to commit" is not a real error — git exits 1 in that case
		fmt.Printf("auto-commit: %s\n", string(out))
	} else {
		fmt.Printf("auto-committed: %s\n", msg)
	}
}

// appendToNotes writes findings to the Obsidian vault. Errors are warnings only — never blocks finish.
func appendToNotes(db *sql.DB, taskID int64, output, status string) {
	cfg, err := config.Load()
	if err != nil || cfg.VaultPath == "" {
		return
	}
	task, err := store.GetTask(db, taskID)
	if err != nil || task == nil {
		fmt.Fprintf(os.Stderr, "warning: notes: could not fetch task: %v\n", err)
		return
	}
	agentID := ""
	if task.ClaimedBy != nil {
		agentID = *task.ClaimedBy
	}
	repoName := vault.RepoName()
	if err := vault.CreateTaskNote(cfg.VaultPath, repoName, taskID, task.Title, task.Description, status, agentID); err != nil {
		fmt.Fprintf(os.Stderr, "warning: vault: could not create task note: %v\n", err)
	}
	if err := vault.AppendFindings(cfg.VaultPath, repoName, taskID, output, status); err != nil {
		fmt.Fprintf(os.Stderr, "warning: vault: could not append findings: %v\n", err)
	}
}
