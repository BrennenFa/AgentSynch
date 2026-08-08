package worker

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"database/sql"

	"agentsynch/internal/config"
	"agentsynch/internal/store"
	"agentsynch/internal/vault"
	"agentsynch/internal/worker/commands/system"
)

// ErrNoTasks is returned by SpawnAgent when no tasks are available to claim.
var ErrNoTasks = errors.New("no available tasks")

// SpawnAgent claims the next available task and launches a tmux window for it.
// Returns ErrNoTasks if no tasks are available, nil on success, or an error on failure.
// Calls system.EnsureSession() internally (idempotent — safe to call from TUI).
func SpawnAgent(db *sql.DB) error {
	if err := system.EnsureSession(); err != nil {
		return fmt.Errorf("tmux error: %w", err)
	}

	cfg, _ := config.Load()
	hostname, _ := os.Hostname()
	agentID := fmt.Sprintf("agent-%s-%d", hostname, os.Getpid())

	task, err := store.Claim(db, agentID, hostname, os.Getpid())
	if err != nil {
		return fmt.Errorf("error claiming task: %w", err)
	}
	if task == nil {
		return ErrNoTasks
	}

	fmt.Printf("claimed task-%d: %s\n", task.ID, task.Title)

	if cfg.VaultPath != "" {
		repoName := vault.RepoName()
		_ = vault.CreateTaskNote(cfg.VaultPath, repoName, task.ID, task.Title, task.Description, task.Status, agentID)
	}

	windowName := fmt.Sprintf("task-%d", task.ID)
	worktreeDir := ""

	if !task.SameBranch {
		slug := system.TitleSlug(task.Title)
		branchName := fmt.Sprintf("task-%d/%s", task.ID, slug)

		created := ""
		for attempt := 1; attempt <= 10; attempt++ {
			candidate := branchName
			if attempt > 1 {
				candidate = fmt.Sprintf("%s-%d", branchName, attempt)
			}
			path := "../worktrees/" + candidate
			if err := system.CreateWorktree(path, candidate); err == nil {
				created = candidate
				worktreeDir = path
				break
			}
		}

		if created == "" {
			fmt.Fprintf(os.Stderr, "warning: could not create worktree for task-%d\n", task.ID)
		} else {
			if err := store.SetBranchName(db, task.ID, created); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not record branch name: %v\n", err)
			}
		}
	}

	windowIndex, err := system.NewWindow(windowName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: could not create tmux window %s: %v\n", windowName, err)
		if err := store.ErrorTask(db, task.ID, fmt.Sprintf("failed to create tmux window: %v", err)); err != nil {
			fmt.Fprintf(os.Stderr, "error: could not mark task-%d as error: %v\n", task.ID, err)
		}
		return fmt.Errorf("could not create tmux window %s: %w", windowName, err)
	}

	if worktreeDir != "" {
		if err := system.SendKeys(windowIndex, fmt.Sprintf("cd %s", worktreeDir)); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not send cd to window %s: %v\n", windowIndex, err)
		}
	}

	claudeCmd := buildClaudePrompt(cfg, task.ID, task.Title)
	if err := system.SendKeys(windowIndex, claudeCmd); err != nil {
		fmt.Fprintf(os.Stderr, "error: could not send claude command to window %s: %v\n", windowIndex, err)
		if err := store.ErrorTask(db, task.ID, fmt.Sprintf("failed to start claude in tmux window: %v", err)); err != nil {
			fmt.Fprintf(os.Stderr, "error: could not mark task-%d as error: %v\n", task.ID, err)
		}
		return fmt.Errorf("could not send claude command to window %s: %w", windowIndex, err)
	}

	if err := store.SetTmuxWindow(db, task.ID, windowName); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not record tmux window: %v\n", err)
	}

	spawnHeartbeat(task.ID)

	fmt.Printf("task-%d running in tmux window %q\n", task.ID, windowName)
	return nil
}

// Run is the main worker loop. Claims tasks and spawns tmux windows for each.
// Sleeps for interval only when no tasks are available or on error; otherwise loops immediately.
func Run(interval time.Duration) {
	db, err := store.Open("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := system.EnsureSession(); err != nil {
		fmt.Fprintf(os.Stderr, "tmux error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("tmux session %q ready — polling for tasks every %s\n", system.SessionName, interval)

	for {
		err := SpawnAgent(db)
		if err == ErrNoTasks {
			time.Sleep(interval)
			continue
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			time.Sleep(interval)
			continue
		}
		// success — immediately try to claim the next task
	}
}

// buildClaudePrompt constructs the claude command string for a task.
// If a vault is configured, codebase context and the task note path are injected.
func buildClaudePrompt(cfg config.Config, taskID int64, title string) string {
	base := fmt.Sprintf("task-%d: %s — follow CLAUDE.md to complete this task.", taskID, title)
	if cfg.VaultPath == "" {
		return fmt.Sprintf(`claude "%s"`, shellEscapeDoubleQuote(base))
	}
	repoName := vault.RepoName()
	codebase, _ := vault.ReadCodebase(cfg.VaultPath, repoName)

	notePath := filepath.Join(cfg.VaultPath, "AgentSynch", repoName, "tasks", fmt.Sprintf("task-%d.md", taskID))

	prompt := base
	if codebase != "" {
		prompt += "\n\nCodebase context:\n" + codebase
	}
	prompt += "\n\nTask note: " + notePath

	return fmt.Sprintf(`claude "%s"`, shellEscapeDoubleQuote(prompt))
}

// shellEscapeDoubleQuote escapes a string for safe embedding inside double quotes in a shell command.
func shellEscapeDoubleQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, `$`, `\$`)
	s = strings.ReplaceAll(s, "`", "\\`")
	return s
}
