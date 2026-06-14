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
		fmt.Fprintln(os.Stderr, "  add        add a new task")
		fmt.Fprintln(os.Stderr, "  list       list all tasks")
		fmt.Fprintln(os.Stderr, "  claim      claim the next available task")
		fmt.Fprintln(os.Stderr, "  finish     mark a claimed task as finished or error")
		fmt.Fprintln(os.Stderr, "  validate   approve or reject a validating task")
		fmt.Fprintln(os.Stderr, "  plan       write a plan for a claimed task")
		fmt.Fprintln(os.Stderr, "  server     start the reaper server")
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
	case "validate":
		commands.Validate()
	case "plan":
		commands.Plan()
	case "server":
		server.Server()
	case "heartbeat":
		server.Heartbeat() // internal — called by background loop spawned in claim.go
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}
