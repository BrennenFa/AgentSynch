package commands

import (
	"fmt"
	"os"
)

func List() {
	store, err := loadTasks()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading tasks: %v\n", err)
		os.Exit(1)
	}
	for _, task := range store.Tasks {
		fmt.Printf("ID: %s, Title: %s, Description: %s, Status: %s\n", task.ID, task.Title, task.Description, task.Status)
	}
}
