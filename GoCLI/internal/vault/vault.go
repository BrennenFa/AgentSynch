package vault

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// frontmatterKeys defines the canonical output order for YAML frontmatter fields.
var frontmatterKeys = []string{"task_id", "repo", "status", "created", "claimed_by", "claimed_at", "finished_at", "branch"}

// parseFrontmatter splits a file's YAML frontmatter from its body.
// Returns an empty map and the full content as body if no valid frontmatter is present.
func parseFrontmatter(content string) (fields map[string]string, body string) {
	fields = map[string]string{}
	if !strings.HasPrefix(content, "---\n") {
		return fields, content
	}
	rest := content[4:] // skip opening "---\n"
	end := strings.Index(rest, "\n---\n")
	if end == -1 {
		return fields, content
	}
	fmBlock := rest[:end]
	body = rest[end+5:] // skip "\n---\n"
	for _, line := range strings.Split(fmBlock, "\n") {
		colon := strings.Index(line, ": ")
		if colon == -1 {
			continue
		}
		key := strings.TrimSpace(line[:colon])
		val := strings.TrimSpace(line[colon+2:])
		fields[key] = val
	}
	return fields, body
}

// serializeFrontmatter outputs frontmatter fields in canonical key order.
// Unknown keys are appended after the known ones.
func serializeFrontmatter(fields map[string]string) string {
	var sb strings.Builder
	written := map[string]bool{}
	for _, k := range frontmatterKeys {
		v := fields[k]
		sb.WriteString(k)
		sb.WriteString(": ")
		sb.WriteString(v)
		sb.WriteByte('\n')
		written[k] = true
	}
	// append any extra keys not in the canonical list
	for k, v := range fields {
		if !written[k] {
			sb.WriteString(k)
			sb.WriteString(": ")
			sb.WriteString(v)
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// UpdateFrontmatter creates or updates the YAML frontmatter block in the task note.
// fields is merged with any existing frontmatter (provided values override existing ones).
// Creates the note file (with empty body) if it does not already exist.
func UpdateFrontmatter(vaultPath, repoName string, taskID int64, fields map[string]string) error {
	path := taskNotePath(vaultPath, repoName, taskID)

	existing := map[string]string{}
	body := ""

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		existing, body = parseFrontmatter(string(data))
	}

	for k, v := range fields {
		existing[k] = v
	}

	fm := serializeFrontmatter(existing)
	content := "---\n" + fm + "---\n" + body

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

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
// Writes YAML frontmatter plus a structured body with Description, Plan, and Findings sections.
func CreateTaskNote(vaultPath, repoName string, taskID int64, title, description, status, agentID string) error {
	path := taskNotePath(vaultPath, repoName, taskID)
	if _, err := os.Stat(path); err == nil {
		// already exists — idempotent
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	fm := serializeFrontmatter(map[string]string{
		"task_id":     fmt.Sprintf("%d", taskID),
		"repo":        repoName,
		"status":      status,
		"created":     now,
		"claimed_by":  agentID,
		"claimed_at":  now,
		"finished_at": "",
		"branch":      "",
	})
	body := fmt.Sprintf("# task-%d: %s\n\n## Description\n%s\n\n## Plan\n(written when `plan` command runs)\n\n---\n\n## Findings\n", taskID, title, description)
	content := "---\n" + fm + "---\n" + body
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

// AppendFindings updates frontmatter status/finished_at and appends findings under ## Findings.
func AppendFindings(vaultPath, repoName string, taskID int64, output, status string) error {
	path := taskNotePath(vaultPath, repoName, taskID)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	fields, body := parseFrontmatter(string(data))

	finishedAt := time.Now().UTC().Format(time.RFC3339)
	fields["status"] = status
	fields["finished_at"] = finishedAt

	findings := fmt.Sprintf("\n**Completed:** %s\n\n%s\n", time.Now().Format("2006-01-02 15:04"), output)
	body += findings

	fm := serializeFrontmatter(fields)
	content := "---\n" + fm + "---\n" + body
	return os.WriteFile(path, []byte(content), 0644)
}
