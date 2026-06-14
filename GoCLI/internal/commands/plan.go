package commands

import (
	"flag"
	"fmt"
	"os"

	"agentsynch/internal/store"
)

func Plan() {
	flags := flag.NewFlagSet("plan", flag.ExitOnError)
	idFlag := flags.Int64("id", 0, "task id")
	planFlag := flags.String("plan", "", "plan for the task")
	flags.Parse(os.Args[2:])

	if *idFlag == 0 || *planFlag == "" {
		fmt.Fprintln(os.Stderr, "usage: plan --id <id> --plan <plan>")
		os.Exit(1)
	}

	db, err := store.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := store.WritePlan(db, *idFlag, *planFlag); err != nil {
		fmt.Fprintf(os.Stderr, "error writing plan: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("plan written for task-%d\n", *idFlag)
}
