package tui

import (
	"database/sql"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"agentsynch/internal/server/tui/logic"
	"agentsynch/internal/server/tui/types"
)

// Run starts the bubbletea TUI, blocking until the user quits.
func Run(db *sql.DB, reapCh chan string) {
	orchIn := make(chan string, 4)
	orchOut := make(chan types.OrchestratorEvent, 8)

	go logic.RunOrchestrator(orchIn, orchOut)

	m := model{
		db:        db,
		reapCh:    reapCh,
		orchInCh:  orchIn,
		orchOutCh: orchOut,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tui error: %v\n", err)
		os.Exit(1)
	}

	close(orchIn) // signals RunOrchestrator to exit cleanly
}
