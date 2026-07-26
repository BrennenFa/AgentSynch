package tui

import (
	"database/sql"

	"agentsynch/internal/objects"
	"agentsynch/internal/server/tui/types"
)

type model struct {
	db         *sql.DB
	tasks      []objects.Task
	cursor     int
	reapMsg    string
	reapCh     chan string
	err        string
	confirming bool // waiting for y/n on delete

	// orchestrator chat
	chatMode    bool
	chatInput   string
	chatHistory []types.ChatEntry
	orchInCh    chan<- string
	orchOutCh   <-chan types.OrchestratorEvent
}
