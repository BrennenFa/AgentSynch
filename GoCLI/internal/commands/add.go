package commands

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"agentsynch/internal/objects"
	"agentsynch/internal/store"
)

func Add() {
	flags := flag.NewFlagSet("add", flag.ExitOnError)

	titleFlag := flags.String("title", "", "task title")
	descFlag := flags.String("description", "", "task description")
	planFlag := flags.String("plan", "", "optional plan for the task")
	dependsOnFlag := flags.String("depends-on", "", "comma-separated task IDs this task depends on (e.g. 1,3)")
	sameBranchFlag := flags.Bool("same-branch", false, "work on current branch; no new branch needed")
	flags.Parse(os.Args[2:])

	var title, description, planInput string

	if *titleFlag != "" && *descFlag != "" {
		title = *titleFlag
		description = *descFlag
		planInput = *planFlag
	} else {
		reader := bufio.NewReader(os.Stdin)

		fmt.Print("title: ")
		title, _ = reader.ReadString('\n')
		title = strings.TrimSpace(title)

		fmt.Print("description: ")
		description, _ = reader.ReadString('\n')
		description = strings.TrimSpace(description)

		fmt.Print("plan (enter to skip): ")
		planInput, _ = reader.ReadString('\n')
		planInput = strings.TrimSpace(planInput)
	}

	// empty plan input = no plan
	var plan *string
	if planInput != "" {
		plan = &planInput
	}

	var dependencies []int64
	// validate dependencies
	if *dependsOnFlag != "" {
		// look at each dependency
		for _, part := range strings.Split(*dependsOnFlag, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			depID, err := strconv.ParseInt(part, 10, 64)
			if err != nil {
				fmt.Fprintf(os.Stderr, "invalid dependency ID %q: must be an integer\n", part)
				os.Exit(1)
			}
			// append each dependency
			dependencies = append(dependencies, depID)
		}
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
		Plan:        plan,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		SameBranch:  *sameBranchFlag,
	}

	id, err := store.AddTask(db, task, dependencies)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error saving task: %v\n", err)
		os.Exit(1)
	}

	if len(dependencies) > 0 {
		fmt.Printf("added task-%d (blocked): %s\n", id, task.Title)
	} else {
		fmt.Printf("added task-%d: %s\n", id, task.Title)
	}
}
