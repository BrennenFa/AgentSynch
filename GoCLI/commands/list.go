package commands

import (
	"fmt"
	"os"

	"agentsynch/store"
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
		fmt.Printf("ID: %s, Title: %s, Description: %s, Status: %s\n", task.ID, task.Title, task.Description, task.Status)
	}
}
