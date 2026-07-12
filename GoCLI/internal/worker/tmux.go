package worker

import (
	"fmt"
	"os/exec"
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
func NewWindow(name string) error {
	return exec.Command("tmux", "new-window", "-t", sessionName, "-n", name).Run()
}

// SendKeys sends a command string to a named window and presses Enter.
func SendKeys(window, cmd string) error {
	target := fmt.Sprintf("%s:%s", sessionName, window)
	return exec.Command("tmux", "send-keys", "-t", target, cmd, "Enter").Run()
}

// SwitchClient switches the tmux client focus to the named window.
func SwitchClient(window string) error {
	target := fmt.Sprintf("%s:%s", sessionName, window)
	return exec.Command("tmux", "switch-client", "-t", target).Run()
}
