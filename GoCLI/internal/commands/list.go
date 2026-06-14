package commands

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"agentsynch/internal/store"
)

func List() {
	db, err := store.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	tasks, err := store.ListTasks(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading tasks: %v\n", err)
		os.Exit(1)
	}

	for _, task := range tasks {
		status := task.Status

		// print out string dependencies
		if task.Status == "blocked" && len(task.Dependencies) > 0 {
			ids := make([]string, len(task.Dependencies))
			for i, id := range task.Dependencies {
				ids[i] = strconv.FormatInt(id, 10)
			}
			// add which ids the task is waiting on
			status += " (waiting on: " + strings.Join(ids, ", ") + ")"
		}
		fmt.Printf("ID: %d, Title: %s, Description: %s, Status: %s\n", task.ID, task.Title, task.Description, status)
	}
}
