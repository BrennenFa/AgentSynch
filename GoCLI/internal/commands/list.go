package commands

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

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

	// clean output with tabwriter and truncate long descriptions
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTITLE\tSTATUS\tDESCRIPTION")
	fmt.Fprintln(w, "--\t-----\t------\t-----------")

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

		// truncate long descriptions for cleaner output
		desc := task.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}

		fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", task.ID, task.Title, status, desc)
	}
	w.Flush()
}
