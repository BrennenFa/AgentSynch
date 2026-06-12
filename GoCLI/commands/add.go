package commands

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"agentsynch/objects"

	"gopkg.in/yaml.v3"
)

const tasksFile = "tasks.yaml"

func loadTasks() (objects.TaskStore, error) {
	// turn tasks yaml into a go struct
	data, err := os.ReadFile(tasksFile)
	if err != nil {
		if os.IsNotExist(err) {
			return objects.TaskStore{}, nil
		}
		return objects.TaskStore{}, err
	}
	var store objects.TaskStore
	err = yaml.Unmarshal(data, &store)
	return store, err
}

func saveTasks(store objects.TaskStore) error {
	data, err := yaml.Marshal(&store)
	if err != nil {
		return err
	}
	header := "# AgentSynch Task Store\n# Statuses: available | claimed | finished | error\n\n"
	return os.WriteFile(tasksFile, append([]byte(header), data...), 0644)
}

func nextID(tasks []objects.Task) string {
	max := 0
	for _, t := range tasks {
		parts := strings.Split(t.ID, "-")
		if len(parts) == 2 {
			n, err := strconv.Atoi(parts[1])
			if err == nil && n > max {
				max = n
			}
		}
	}
	return fmt.Sprintf("task-%03d", max+1)
}

func Add() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("title: ")
	title, _ := reader.ReadString('\n')
	title = strings.TrimSpace(title)

	fmt.Print("description: ")
	description, _ := reader.ReadString('\n')
	description = strings.TrimSpace(description)

	// get current task store
	store, err := loadTasks()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading tasks: %v\n", err)
		os.Exit(1)
	}

	// create new task object
	task := objects.Task{
		ID:          nextID(store.Tasks),
		Title:       title,
		Description: description,
		Status:      "available",
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	store.Tasks = append(store.Tasks, task)

	// write tasks
	err = saveTasks(store)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error saving tasks: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("added %s: %s\n", task.ID, task.Title)
}
