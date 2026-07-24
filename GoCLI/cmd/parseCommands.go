package main

import (
	"fmt"
	"os"

	"agentsynch/internal/commands"
	"agentsynch/internal/server"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: agentsynch <command>")
		fmt.Fprintln(os.Stderr, "commands:")
		fmt.Fprintln(os.Stderr, "  add          add a new task")
		fmt.Fprintln(os.Stderr, "  list         list all tasks")
		fmt.Fprintln(os.Stderr, "  claim        claim the next available task")
		fmt.Fprintln(os.Stderr, "  finish       mark a claimed task as finished or error")
		fmt.Fprintln(os.Stderr, "  plan         write a plan for a claimed task")
		fmt.Fprintln(os.Stderr, "  set-branch   record the branch created for a claimed task")
		fmt.Fprintln(os.Stderr, "  archive      manually archive a finished or error task")
		fmt.Fprintln(os.Stderr, "  tui          open the live dashboard (runs reaper + github worker)")
		fmt.Fprintln(os.Stderr, "  worker       poll for tasks and run them in tmux windows")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "add":
		commands.Add()
	case "list":
		commands.List()
	case "claim":
		commands.Claim()
	case "finish":
		commands.Finish()
	case "plan":
		commands.Plan()
	case "set-branch":
		commands.SetBranch()
	case "archive":
		commands.Archive()
	case "tui":
		server.TUI()
	case "worker":
		commands.Worker()
	case "heartbeat":
		server.Heartbeat() // internal — called by background loop spawned in claim.go
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}
