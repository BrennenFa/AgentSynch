package vault

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// RepoName returns the base name of the git repository root.
func RepoName() string {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "unknown"
	}
	return filepath.Base(strings.TrimSpace(string(out)))
}

// ReadCodebase reads <vault>/AgentSynch/<repo>/codebase.md.
// Returns empty string (not error) if the file does not exist.
func ReadCodebase(vaultPath, repoName string) (string, error) {
	path := filepath.Join(vaultPath, "AgentSynch", repoName, "codebase.md")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func taskNotePath(vaultPath, repoName string, taskID int64) string {
	return filepath.Join(vaultPath, "AgentSynch", repoName, "tasks", fmt.Sprintf("task-%d.md", taskID))
}

// CreateTaskNote creates the task note file if it does not already exist (idempotent).
func CreateTaskNote(vaultPath, repoName string, taskID int64, title, description, status, agentID string) error {
	path := taskNotePath(vaultPath, repoName, taskID)
	if _, err := os.Stat(path); err == nil {
		// already exists — idempotent
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	started := time.Now().Format("2006-01-02 15:04")
	content := fmt.Sprintf("# task-%d: %s\n\n**Status:** %s\n**Agent:** %s\n**Started:** %s\n\n## Description\n%s\n\n## Plan\n\n---\n\n## Findings\n", taskID, title, status, agentID, started, description)
	return os.WriteFile(path, []byte(content), 0644)
}

// ReadPlan returns the content of the ## Plan section in the task note.
// Returns empty string if the vault note or section does not exist.
func ReadPlan(vaultPath, repoName string, taskID int64) string {
	path := taskNotePath(vaultPath, repoName, taskID)
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	content := string(data)

	planHeader := "## Plan\n"
	sep := "\n---\n"


	// find start location
	headerIndex := strings.Index(content, planHeader)
	if headerIndex == -1 {
		return ""
	}
	startIndex := headerIndex + len(planHeader)

	sepIdx := strings.Index(content[startIndex:], sep)

	// find the text lines needed
	if sepIdx == -1 {
		return strings.TrimSpace(content[startIndex:])
	}
	return strings.TrimSpace(content[startIndex : startIndex+sepIdx])
}

// AppendFindings updates the status line and appends findings under ## Findings.
func AppendFindings(vaultPath, repoName string, taskID int64, output, status string) error {
	path := taskNotePath(vaultPath, repoName, taskID)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(data)

	// update **Status:** line
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "**Status:**") {
			lines[i] = fmt.Sprintf("**Status:** %s", status)
			break
		}
	}
	content = strings.Join(lines, "\n")

	// append findings after the existing content
	findings := fmt.Sprintf("\n**Completed:** %s\n\n%s\n", time.Now().Format("2006-01-02 15:04"), output)
	content += findings
	return os.WriteFile(path, []byte(content), 0644)
}
