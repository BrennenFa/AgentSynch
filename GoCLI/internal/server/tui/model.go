package tui

import (
	"database/sql"

	"agentsynch/internal/objects"
)

type model struct {
	db              *sql.DB
	tasks           []objects.Task
	cursor          int   // tasks tab cursor
	agentCursor     int   // agents tab cursor
	selectedID      int64 // ID of the task under the tasks-tab cursor (0 = none); keeps selection stable across reloads
	selectedAgentID int64 // ID of the task under the agents-tab cursor (0 = none); keeps selection stable across reloads
	activeTab       int   // 0 = tasks, 1 = agents
	reapMsg         string
	reapCh          chan string
	err             string
	confirming      bool   // waiting for y/n on delete
	confirmingKill  bool   // waiting for y/n on kill agent
	preview         string // last tmux capture-pane output
	width           int    // terminal width from WindowSizeMsg
	height          int    // terminal height from WindowSizeMsg
	// add task form
	addingTask bool   // add-task form is open
	formField  int    // 0 = title, 1 = description
	titleInput string // title field value
	descInput  string // description field value
}
