package worker

import (
	"os/exec"
	"regexp"
	"strings"
)

var nonAlphanumDash = regexp.MustCompile(`[^a-z0-9-]+`)

// titleSlug converts a task title to a lowercase hyphenated slug.
// Duplicated from commands/git.go to avoid import cycle.
func titleSlug(title string) string {
	s := strings.ToLower(title)
	s = strings.ReplaceAll(s, " ", "-")
	s = nonAlphanumDash.ReplaceAllString(s, "")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}

// createWorktree creates a new branch in a separate working tree directory.
// Duplicated from commands/git.go to avoid import cycle.
func createWorktree(path, branch string) error {
	return exec.Command("git", "worktree", "add", path, "-b", branch).Run()
}
