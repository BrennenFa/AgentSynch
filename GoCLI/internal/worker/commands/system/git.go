package system

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"agentsynch/internal/config"
	"agentsynch/internal/objects"
	"agentsynch/internal/vault"
)

func CheckoutNewBranch(name string) error {
	return exec.Command("git", "checkout", "-b", name).Run()
}

func CreateWorktree(path, branch string) error {
	return exec.Command("git", "worktree", "add", path, "-b", branch).Run()
}

func PushBranch(name string) error {
	return exec.Command("git", "push", "-u", "origin", name).Run()
}

func CreatePR(task objects.Task) (string, error) {
	// read plan from vault (vault is source of truth)
	plan := ""
	if cfg, err := config.Load(); err == nil && cfg.VaultPath != "" {
		plan = vault.ReadPlan(cfg.VaultPath, vault.RepoName(), task.ID)
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

func CreateIssue(task objects.Task) (string, error) {
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

// TitleSlug converts a task title to a lowercase hyphenated slug for branch names.
func TitleSlug(title string) string {
	s := strings.ToLower(title)
	s = strings.ReplaceAll(s, " ", "-")
	s = regexp.MustCompile(`[^a-z0-9-]+`).ReplaceAllString(s, "")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	return s
}
