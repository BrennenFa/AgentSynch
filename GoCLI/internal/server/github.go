package server

import (
	"bytes"
	"database/sql"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"agentsynch/internal/objects"
	"agentsynch/internal/store"
)

const githubInterval = 10 * time.Second

// githubWorker runs in a goroutine and processes finished/error tasks into github
func githubWorker(db *sql.DB) {
	runGithubPass(db)
	ticker := time.NewTicker(githubInterval)
	defer ticker.Stop()

	// run githubpass function
	for range ticker.C {
		runGithubPass(db)
	}
}

// runGithubPass performs a single pass: PRs for finished tasks, Issues for error tasks.
func runGithubPass(db *sql.DB) {
	// process finished tasks
	finished, err := store.ListFinishedForGH(db)
	if err != nil {
		log.Printf("github worker: error listing finished tasks: %v", err)
	}

	// iterate through finished tasks, create PR and archive each one
	for _, task := range finished {
		var url string
		// only create a PR if the agent worked on a dedicated branch
		if task.BranchName != nil && *task.BranchName != "" {
			url, err = createPR(task)
			if err != nil {
				log.Printf("github worker: error creating PR for task-%d: %v", task.ID, err)
				continue
			}
		} else {
			// if no branch, assume same branch pr
			url = "same-branch"
		}

		// archive the task
		if err := store.SetDbGit(db, task.ID, url); err != nil {
			log.Printf("github worker: error archiving task-%d: %v", task.ID, err)
		}
	}

	// process error tasks
	errors, err := store.ListErrorsForGH(db)
	if err != nil {
		log.Printf("github worker: error listing error tasks: %v", err)
	}
	for _, task := range errors {
		url, err := createIssue(task)
		if err != nil {
			log.Printf("github worker: error creating issue for task-%d: %v", task.ID, err)
			continue
		}

		// archive the task, recording the issue URL
		if err := store.SetDbGit(db, task.ID, url); err != nil {
			log.Printf("github worker: error archiving task-%d: %v", task.ID, err)
		}
	}
}

// createPR creates a GitHub PR for a finished new-branch task. Returns the PR URL.
func createPR(task objects.Task) (string, error) {
	plan := ""
	if task.Plan != nil {
		plan = *task.Plan
	}
	output := ""
	if task.Output != nil {
		output = *task.Output
	}
	branchName := *task.BranchName

	body := fmt.Sprintf("## Description\n%s\n\n## Plan\n%s\n\n## Output\n%s",
		task.Description, plan, output)

	var stdout bytes.Buffer
	cmd := exec.Command("gh", "pr", "create",
		"--title", fmt.Sprintf("task-%d: %s", task.ID, task.Title),
		"--body", body,
		"--base", "main",
		"--head", branchName,
	)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

// createIssue creates a GitHub Issue for a failed task. Returns the issue URL.
func createIssue(task objects.Task) (string, error) {
	errMsg := ""
	if task.Error != nil {
		errMsg = *task.Error
	}

	body := fmt.Sprintf("## Description\n%s\n\n## Error\n%s",
		task.Description, errMsg)

	var stdout bytes.Buffer
	cmd := exec.Command("gh", "issue", "create",
		"--title", fmt.Sprintf("task-%d failed: %s", task.ID, task.Title),
		"--body", body,
		"--label", "bug",
	)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}
