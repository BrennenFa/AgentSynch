package commands

import (
	"os/exec"
	"regexp"
	"strings"
)

// checkoutNewBranch creates and checks out a new git branch.
func checkoutNewBranch(name string) error {
	return exec.Command("git", "checkout", "-b", name).Run()
}

// pushBranch pushes a branch to origin, setting upstream tracking.
func pushBranch(name string) error {
	return exec.Command("git", "push", "-u", "origin", name).Run()
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
