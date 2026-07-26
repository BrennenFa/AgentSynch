package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"agentsynch/internal/objects"
	"agentsynch/internal/server/tui/types"
)

// RenderTaskTable renders the main task dashboard.
func RenderTaskTable(tasks []objects.Task, cursor int, confirming bool, reapMsg, err string) string {
	var b strings.Builder

	b.WriteString(Bold.Render("AgentSynch Dashboard") + "\n\n")

	if len(tasks) == 0 {
		b.WriteString("no tasks\n")
	} else {
		b.WriteString(Bold.Render(fmt.Sprintf(
			"  %-4s  %-28s  %-11s  %-18s  %-8s  %-6s  %-10s",
			"ID", "Title", "Status", "Agent", "Heartbeat", "Tries", "Window",
		)) + "\n")
		b.WriteString("  " + strings.Repeat("─", 95) + "\n")

		for i, t := range tasks {
			prefix := "  "
			if i == cursor {
				prefix = CursorCol.Render("> ")
			}

			line := fmt.Sprintf("%s%-4d  %-28s  %s  %-18s  %-8s  %-6d  %-10s",
				prefix,
				t.ID,
				trunc(t.Title, 28),
				colorStatus(t.Status),
				trunc(deref(t.ClaimedBy, "-"), 18),
				heartbeatStr(t.HeartbeatAt),
				t.Attempts,
				trunc(deref(t.TmuxWindow, "-"), 10),
			)
			b.WriteString(line + "\n")
		}
	}

	// active agents panel
	claimed := byStatus(tasks, "claimed")
	if len(claimed) > 0 {
		b.WriteString("\n" + Bold.Render("Active agents:") + "\n")
		for _, t := range claimed {
			b.WriteString(fmt.Sprintf("  task-%d: %s  agent=%s  window=%s\n",
				t.ID, t.Title,
				deref(t.ClaimedBy, "?"),
				deref(t.TmuxWindow, "none"),
			))
		}
	}

	// reaper panel
	b.WriteString("\n" + Bold.Render("Reaper:") + "\n")
	if reapMsg == "" {
		b.WriteString("  not run yet\n")
	} else {
		b.WriteString("  " + reapMsg + "\n")
	}

	if confirming {
		// caller will append the specific task confirm line
	} else {
		b.WriteString("\nj/k: navigate  a: attach  d: delete  c: chat  q: quit")
		if err != "" {
			b.WriteString("  " + ErrStyle.Render("err: "+err))
		}
	}

	return b.String()
}

// RenderChat renders the orchestrator chat panel.
func RenderChat(state types.ChatState) string {
	var b strings.Builder

	b.WriteString("\n" + Bold.Render("Claude Orchestrator") + "\n")

	const maxLines = 8
	start := 0
	if len(state.History) > maxLines {
		start = len(state.History) - maxLines
	}
	for _, entry := range state.History[start:] {
		switch entry.From {
		case "you":
			b.WriteString(ChatYou.Render("you") + ": " + entry.Text + "\n")
		case "claude":
			b.WriteString(ChatClaude.Render("claude") + ": " + entry.Text + "\n")
		default: // "system"
			b.WriteString(ChatSystem.Render("  "+entry.Text) + "\n")
		}
	}

	if state.Mode {
		b.WriteString(ChatPrompt.Render("> ") + state.Input + "█\n")
		b.WriteString(ChatSystem.Render("esc: exit chat  enter: send"))
	} else {
		b.WriteString(ChatSystem.Render("> (press c to chat with Claude)"))
	}

	return b.String()
}

func colorStatus(s string) string {
	style, ok := StatusColor[s]
	if !ok {
		style = lipgloss.NewStyle()
	}
	return style.Width(11).Render(s)
}

func heartbeatStr(hb *string) string {
	if hb == nil {
		return fmt.Sprintf("%-8s", "-")
	}
	t, err := time.Parse(time.RFC3339, *hb)
	if err != nil {
		return fmt.Sprintf("%-8s", "-")
	}
	age := time.Since(t)
	s := fmt.Sprintf("%-8s", formatAge(age))
	switch {
	case age < 5*time.Minute:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render(s)
	case age < 10*time.Minute:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Render(s)
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render(s)
	}
}

func formatAge(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh", int(d.Hours()))
}

func deref(s *string, def string) string {
	if s == nil {
		return def
	}
	return *s
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func byStatus(tasks []objects.Task, status string) []objects.Task {
	var out []objects.Task
	for _, t := range tasks {
		if t.Status == status {
			out = append(out, t)
		}
	}
	return out
}
