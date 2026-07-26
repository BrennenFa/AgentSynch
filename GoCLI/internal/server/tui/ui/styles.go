package ui

import "github.com/charmbracelet/lipgloss"

var (
	Bold      = lipgloss.NewStyle().Bold(true)
	CursorCol = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	ErrStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))

	StatusColor = map[string]lipgloss.Style{
		"available": lipgloss.NewStyle().Foreground(lipgloss.Color("2")),
		"claimed":   lipgloss.NewStyle().Foreground(lipgloss.Color("3")),
		"finished":  lipgloss.NewStyle().Foreground(lipgloss.Color("4")),
		"error":     lipgloss.NewStyle().Foreground(lipgloss.Color("1")),
		"archived":  lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
	}

	// chat panel styles
	ChatYou    = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	ChatClaude = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	ChatSystem = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	ChatPrompt = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
)
