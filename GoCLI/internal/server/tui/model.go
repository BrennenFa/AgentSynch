package tui

import (
	"database/sql"

	"agentsynch/internal/objects"
)

type model struct {
	db             *sql.DB
	tasks          []objects.Task
	cursor         int // tasks tab cursor
	agentCursor    int // agents tab cursor
	activeTab      int // 0 = tasks, 1 = agents
	reapMsg        string
	reapCh         chan string
	err            string
	confirming     bool // waiting for y/n on delete
	confirmingKill bool // waiting for y/n on kill agent
	preview        string // last tmux capture-pane output
	width          int    // terminal width from WindowSizeMsg
	height         int    // terminal height from WindowSizeMsg
}
