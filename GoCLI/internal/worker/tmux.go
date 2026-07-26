package worker

import (
	"fmt"
	"os/exec"
	"strings"
)

const sessionName = "agentsynch"

// EnsureSession checks tmux is available and creates the agentsynch session if missing.
func EnsureSession() error {
	if _, err := exec.LookPath("tmux"); err != nil {
		return fmt.Errorf("tmux not found in PATH: install tmux to use the worker command")
	}

	// if session already exists, nothing to do
	if exec.Command("tmux", "has-session", "-t", sessionName).Run() == nil {
		return nil
	}

	// create session with window 0 named "dashboard"
	if err := exec.Command("tmux", "new-session", "-d", "-s", sessionName, "-n", "dashboard").Run(); err != nil {
		return fmt.Errorf("could not create tmux session %q: %w", sessionName, err)
	}
	return nil
}

// NewWindow creates a new named window in the agentsynch session.
// If a window with that name already exists, it is killed first.
// Returns the window index so callers can target it unambiguously.
func NewWindow(name string) (string, error) {
	target := fmt.Sprintf("%s:%s", sessionName, name)
	exec.Command("tmux", "kill-window", "-t", target).Run() // ignore error if it doesn't exist
	out, err := exec.Command("tmux", "new-window", "-t", sessionName, "-n", name, "-P", "-F", "#{window_index}").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// SendKeys sends a command string to a window (by index) and presses Enter.
func SendKeys(windowIndex, cmd string) error {
	target := fmt.Sprintf("%s:%s", sessionName, windowIndex)
	return exec.Command("tmux", "send-keys", "-t", target, cmd, "Enter").Run()
}

// SwitchClient switches the tmux client focus to the named window.
func SwitchClient(window string) error {
	target := fmt.Sprintf("%s:%s", sessionName, window)
	return exec.Command("tmux", "switch-client", "-t", target).Run()
}
