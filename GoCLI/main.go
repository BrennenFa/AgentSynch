package main

import (
	"fmt"
	"os"

	"agentsynch/commands"
)

func main() {
	// ensure at least 2 args
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: agentsynch <command>")
		fmt.Fprintln(os.Stderr, "commands:")
		fmt.Fprintln(os.Stderr, "  add    add a new task")
		fmt.Fprintln(os.Stderr, "  list   list all tasks")
		fmt.Fprintln(os.Stderr, "  claim  claim the next available task")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "add":
		commands.Add()
	case "list":
		commands.List()
	case "claim":
		commands.Claim()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}
