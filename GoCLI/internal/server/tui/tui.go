package tui

import (
	"database/sql"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"agentsynch/internal/config"
	"agentsynch/internal/vault"
)

// Run starts the bubbletea TUI, blocking until the user quits.
func Run(db *sql.DB, reapCh chan string) {
	cfg, _ := config.Load()
	m := model{db: db, reapCh: reapCh, vaultPath: cfg.VaultPath, repoName: vault.RepoName()}
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tui error: %v\n", err)
		os.Exit(1)
	}
}
