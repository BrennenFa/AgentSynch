package commands

import (
	"flag"
	"fmt"
	"os"

	"agentsynch/internal/store"
)

func SetBranch() {
	flags := flag.NewFlagSet("set-branch", flag.ExitOnError)
	idFlag := flags.Int64("id", 0, "task ID")
	nameFlag := flags.String("name", "", "branch name to record")
	flags.Parse(os.Args[2:])

	if *idFlag == 0 {
		fmt.Fprintln(os.Stderr, "error: --id is required")
		os.Exit(1)
	}
	if *nameFlag == "" {
		fmt.Fprintln(os.Stderr, "error: --name is required")
		os.Exit(1)
	}

	db, err := store.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := store.SetBranchName(db, *idFlag, *nameFlag); err != nil {
		fmt.Fprintf(os.Stderr, "error setting branch: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("set branch for task-%d: %s\n", *idFlag, *nameFlag)
}
