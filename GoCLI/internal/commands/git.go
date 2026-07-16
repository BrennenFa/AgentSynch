package commands

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"agentsynch/internal/objects"
)

// checkoutNewBranch creates and checks out a new git branch.
func checkoutNewBranch(name string) error {
	return exec.Command("git", "checkout", "-b", name).Run()
}

// createWorktree creates a new branch in a separate working tree directory.
func createWorktree(path, branch string) error {
	return exec.Command("git", "worktree", "add", path, "-b", branch).Run()
}

// pushBranch pushes a branch to origin, setting upstream tracking.
func pushBranch(name string) error {
	return exec.Command("git", "push", "-u", "origin", name).Run()
}

// createPR opens a GitHub PR for a finished task. Returns the PR URL.
func createPR(task objects.Task) (string, error) {
	plan := ""
	if task.Plan != nil {
		plan = *task.Plan
	}
	output := ""
	if task.Output != nil {
		output = *task.Output
	}
	body := fmt.Sprintf("## Description\n%s\n\n## Plan\n%s\n\n## Output\n%s",
		task.Description, plan, output)

	var stdout bytes.Buffer
	cmd := exec.Command("gh", "pr", "create",
		"--title", fmt.Sprintf("task-%d: %s", task.ID, task.Title),
		"--body", body,
		"--base", "main",
		"--head", *task.BranchName,
	)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

// createIssue opens a GitHub Issue for a failed task. Returns the issue URL.
func createIssue(task objects.Task) (string, error) {
	errMsg := ""
	if task.Error != nil {
		errMsg = *task.Error
	}
	body := fmt.Sprintf("## Description\n%s\n\n## Error\n%s", task.Description, errMsg)

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

var nonAlphanumDash = regexp.MustCompile(`[^a-z0-9-]+`)

// titleSlug converts a task title to a lowercase hyphenated slug for branch names.
func titleSlug(title string) string {
	s := strings.ToLower(title)
	s = strings.ReplaceAll(s, " ", "-")
	s = nonAlphanumDash.ReplaceAllString(s, "")
	// collapse consecutive hyphens
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	return s
}
