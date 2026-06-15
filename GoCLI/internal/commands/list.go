package commands

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"agentsynch/internal/store"
)

// List outputs task data as JSON for agent consumption.
// Flags:
//
//	--id <n>       fetch a single task by ID
//	--status <s>   filter by status (available, claimed, validating, finished, error, blocked, archived)
//	--all          include archived tasks (ignored when --id or --status is set)
func List() {
	flags := flag.NewFlagSet("list", flag.ExitOnError)
	idFlag := flags.Int64("id", 0, "fetch a single task by ID")
	statusFlag := flags.String("status", "", "filter tasks by status")
	allFlag := flags.Bool("all", false, "include archived tasks")
	flags.Parse(os.Args[2:])

	db, err := store.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	// single-task lookup
	if *idFlag != 0 {
		task, err := store.GetTask(db, *idFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading task: %v\n", err)
			os.Exit(1)
		}
		if task == nil {
			fmt.Fprintf(os.Stderr, "task-%d not found\n", *idFlag)
			os.Exit(1)
		}
		enc.Encode(task)
		return
	}

	// filtered or full list
	var tasks interface{}
	if *statusFlag != "" {
		tasks, err = store.ListTasksByStatus(db, *statusFlag)
	} else if *allFlag {
		tasks, err = store.ListAllTasks(db)
	} else {
		tasks, err = store.ListTasks(db)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading tasks: %v\n", err)
		os.Exit(1)
	}

	enc.Encode(tasks)
}
