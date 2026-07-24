package commands

import (
	"flag"
	"os"
	"time"

	"agentsynch/internal/worker"
)

func Worker() {
	fs := flag.NewFlagSet("worker", flag.ExitOnError)
	interval := fs.Duration("interval", 5*time.Second, "sleep duration between polls when no tasks are available")
	fs.Parse(os.Args[2:])

	worker.Run(*interval)
}
