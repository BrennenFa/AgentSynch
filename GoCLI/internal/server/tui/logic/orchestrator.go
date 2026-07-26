package logic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"agentsynch/internal/server/tui/types"
	"agentsynch/internal/worker/commands/system"
)

// claudeJSONResponse is the shape of claude --output-format json output.
type claudeJSONResponse struct {
	SessionID string `json:"session_id"`
	Result    string `json:"result"`
}

// RunOrchestrator is the long-lived goroutine that owns the master Claude session.
// It reads user messages from in, calls the claude CLI, and sends events to out.
// It exits cleanly when in is closed.
func RunOrchestrator(in <-chan string, out chan<- types.OrchestratorEvent) {
	var sessionID string
	spawnCounter := 0

	for userMsg := range in {
		resp, sid, err := callClaude(sessionID, userMsg)
		if err != nil {
			out <- types.OrchestratorEvent{Err: err.Error()}
			continue
		}
		sessionID = sid

		// parse SPAWN: directives out of the response
		var spawnMsgs []string
		var cleanLines []string
		for _, line := range strings.Split(resp, "\n") {
			if strings.HasPrefix(line, "SPAWN:") {
				desc := strings.TrimSpace(strings.TrimPrefix(line, "SPAWN:"))
				windowName := fmt.Sprintf("orchestrator-%d", spawnCounter)
				spawnCounter++
				spawnMsgs = append(spawnMsgs, spawnAgent(windowName, desc))
			} else {
				cleanLines = append(cleanLines, line)
			}
		}

		out <- types.OrchestratorEvent{
			Response:  strings.TrimSpace(strings.Join(cleanLines, "\n")),
			SpawnMsgs: spawnMsgs,
		}
	}
}

// callClaude runs claude --print --output-format json as a subprocess.
// On first call sessionID should be empty; subsequent calls pass --resume <id>.
// Returns (responseText, newSessionID, error).
func callClaude(sessionID, message string) (string, string, error) {
	args := []string{"--print", "--output-format", "json", "-p", message}
	if sessionID != "" {
		args = append([]string{"--resume", sessionID}, args...)
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command("claude", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("claude: %w — %s", err, strings.TrimSpace(stderr.String()))
	}

	var r claudeJSONResponse
	if err := json.Unmarshal(stdout.Bytes(), &r); err != nil {
		return "", "", fmt.Errorf("parse: %w — raw: %s", err, stdout.String())
	}
	return r.Result, r.SessionID, nil
}

// spawnAgent creates a tmux window and starts claude inside it.
// Returns a human-readable status string for the chat log.
func spawnAgent(windowName, taskDescription string) string {
	idx, err := system.NewWindow(windowName)
	if err != nil {
		return fmt.Sprintf("[spawn failed for %q: %v]", windowName, err)
	}
	claudeCmd := fmt.Sprintf(`claude "%s"`, taskDescription)
	if err := system.SendKeys(idx, claudeCmd); err != nil {
		return fmt.Sprintf("[window %s created but send failed: %v]", windowName, err)
	}
	return fmt.Sprintf("[spawned %s: %s]", windowName, taskDescription)
}
