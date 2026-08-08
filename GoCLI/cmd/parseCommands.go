package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"agentsynch/internal/server"
	"agentsynch/internal/worker"
	agentcmds "agentsynch/internal/worker/commands/db"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: agentsynch <command>")
		fmt.Fprintln(os.Stderr, "commands:")
		fmt.Fprintln(os.Stderr, "  add          add a new task")
		fmt.Fprintln(os.Stderr, "  claim        claim the next available task")
		fmt.Fprintln(os.Stderr, "  config       view or set configuration (e.g. vault path)")
		fmt.Fprintln(os.Stderr, "  finish       mark a claimed task as finished or error")
		fmt.Fprintln(os.Stderr, "  set-branch   record the branch created for a claimed task")
		fmt.Fprintln(os.Stderr, "  tui          open the live dashboard (runs reaper + github worker)")
		fmt.Fprintln(os.Stderr, "  worker       poll for tasks and run them in tmux windows")
		os.Exit(1)
	}

	// All subcommands except `config` assume they're running inside a project
	// repo checkout with a CLAUDE.md at its root (git worktrees, agent prompts,
	// etc. all depend on this). Fail fast with a clear message instead of
	// letting it crash deep in a subcommand.
	if os.Args[1] != "config" {
		out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
		if err != nil {
			fmt.Fprintln(os.Stderr, "agentsynch: not inside a git repository — run this command from within a project checkout")
			os.Exit(1)
		}
		repoRoot := strings.TrimSpace(string(out))
		if _, err := os.Stat(filepath.Join(repoRoot, "CLAUDE.md")); err != nil {
			fmt.Fprintln(os.Stderr, "agentsynch: no CLAUDE.md found at repo root ("+repoRoot+") — this command requires a CLAUDE.md-configured project")
			os.Exit(1)
		}
	}

	switch os.Args[1] {
	case "add":
		agentcmds.Add()
	case "claim":
		agentcmds.Claim()
	case "config":
		agentcmds.Config()
	case "finish":
		agentcmds.Finish()
	case "set-branch":
		agentcmds.SetBranch()
	case "tui":
		server.TUI()
	case "worker":
		fs := flag.NewFlagSet("worker", flag.ExitOnError)
		interval := fs.Duration("interval", 5*time.Second, "sleep duration between polls when no tasks are available")
		fs.Parse(os.Args[2:])
		worker.Run(*interval)
	case "heartbeat":
		agentcmds.Heartbeat() // internal — called by background loop spawned in worker
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}
