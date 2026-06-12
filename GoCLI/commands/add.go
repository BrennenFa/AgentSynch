package commands

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"agentsynch/objects"
	"agentsynch/store"
)

func Add() {
	flags := flag.NewFlagSet("add", flag.ExitOnError)

	titleFlag := flags.String("title", "", "task title")
	descFlag := flags.String("description", "", "task description")
	flags.Parse(os.Args[2:])

	var title, description string

	if *titleFlag != "" && *descFlag != "" {
		title = *titleFlag
		description = *descFlag
	} else {
		reader := bufio.NewReader(os.Stdin)

		fmt.Print("title: ")
		title, _ = reader.ReadString('\n')
		title = strings.TrimSpace(title)

		fmt.Print("description: ")
		description, _ = reader.ReadString('\n')
		description = strings.TrimSpace(description)
	}

	db, err := store.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	task := objects.Task{
		Title:       title,
		Description: description,
		Status:      "available",
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	id, err := store.AddTask(db, task)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error saving task: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("added task-%d: %s\n", id, task.Title)
}
