package worker

import (
	"fmt"
	"os"
	"time"

	"agentsynch/internal/store"
	"agentsynch/internal/worker/commands/system"
)

// Run is the main worker loop. Claims tasks and spawns tmux windows for each.
// Sleeps for interval only when no tasks are available; otherwise loops immediately.
func Run(interval time.Duration) {
	db, err := store.Open()
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

	hostname, _ := os.Hostname()

	for {
		agentID := fmt.Sprintf("agent-%s-%d", hostname, os.Getpid())

		task, err := store.Claim(db, agentID, hostname, os.Getpid())
		if err != nil {
			fmt.Fprintf(os.Stderr, "error claiming task: %v\n", err)
			time.Sleep(interval)
			continue
		}

		if task == nil {
			time.Sleep(interval)
			continue
		}

		fmt.Printf("claimed task-%d: %s\n", task.ID, task.Title)

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
			continue
		}

		if worktreeDir != "" {
			if err := system.SendKeys(windowIndex, fmt.Sprintf("cd %s", worktreeDir)); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not send cd to window %s: %v\n", windowIndex, err)
			}
		}

		claudeCmd := fmt.Sprintf(`claude "task-%d: %s — follow CLAUDE.md to complete this task."`, task.ID, task.Title)
		if err := system.SendKeys(windowIndex, claudeCmd); err != nil {
			fmt.Fprintf(os.Stderr, "error: could not send claude command to window %s: %v\n", windowIndex, err)
			if err := store.ErrorTask(db, task.ID, fmt.Sprintf("failed to start claude in tmux window: %v", err)); err != nil {
				fmt.Fprintf(os.Stderr, "error: could not mark task-%d as error: %v\n", task.ID, err)
			}
			continue
		}

		if err := store.SetTmuxWindow(db, task.ID, windowName); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not record tmux window: %v\n", err)
		}

		spawnHeartbeat(task.ID)

		fmt.Printf("task-%d running in tmux window %q\n", task.ID, windowName)
		// no sleep — immediately try to claim the next task
	}
}
