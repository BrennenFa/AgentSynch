package commands

import (
	"flag"
	"fmt"
	"os"

	"agentsynch/internal/store"
)

func Validate() {
	flags := flag.NewFlagSet("validate", flag.ExitOnError)
	idFlag := flags.Int64("id", 0, "task id")
	rejectFlag := flags.String("reject", "", "reject the task with a reason; omit to approve")
	flags.Parse(os.Args[2:])

	if *idFlag == 0 {
		fmt.Fprintln(os.Stderr, "usage: validate --id <id>")
		fmt.Fprintln(os.Stderr, "       validate --id <id> --reject \"reason\"")
		os.Exit(1)
	}

	db, err := store.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	approve := *rejectFlag == ""
	if err := store.ValidateTask(db, *idFlag, approve, *rejectFlag); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if approve {
		fmt.Printf("task-%d approved and marked as finished\n", *idFlag)
	} else {
		fmt.Printf("task-%d rejected and returned to available\n", *idFlag)
	}
}
