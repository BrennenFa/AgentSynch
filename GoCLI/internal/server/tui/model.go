package tui

import (
	"database/sql"

	"agentsynch/internal/objects"
)

type model struct {
	db         *sql.DB
	tasks      []objects.Task
	cursor     int
	reapMsg    string
	reapCh     chan string
	err        string
	confirming bool // waiting for y/n on delete
}
