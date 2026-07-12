package tui

import (
	"database/sql"
	"time"

	"agentsynch/internal/objects"
)

type model struct {
	db       *sql.DB
	tasks    []objects.Task
	cursor   int
	lastReap time.Time
	err      string
}
