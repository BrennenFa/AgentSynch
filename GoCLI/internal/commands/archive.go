package commands

import (
	"flag"
	"fmt"
	"os"

	"agentsynch/internal/store"
)

func Archive() {
	flags := flag.NewFlagSet("archive", flag.ExitOnError)
	idFlag := flags.Int64("id", 0, "task ID to archive")
	flags.Parse(os.Args[2:])

	if *idFlag == 0 {
		fmt.Fprintln(os.Stderr, "error: --id is required")
		os.Exit(1)
	}

	db, err := store.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := store.ArchiveTask(db, *idFlag); err != nil {
		fmt.Fprintf(os.Stderr, "error archiving task: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("archived task-%d\n", *idFlag)
}
