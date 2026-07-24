package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"agentsynch/internal/objects"
)

var (
	bold      = lipgloss.NewStyle().Bold(true)
	cursorCol = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	errStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))

	statusColor = map[string]lipgloss.Style{
		"available": lipgloss.NewStyle().Foreground(lipgloss.Color("2")),
		"claimed":   lipgloss.NewStyle().Foreground(lipgloss.Color("3")),
		"finished":  lipgloss.NewStyle().Foreground(lipgloss.Color("4")),
		"error":     lipgloss.NewStyle().Foreground(lipgloss.Color("1")),
		"blocked":   lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		"archived":  lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
	}
)

func (m model) View() string {
	var b strings.Builder

	b.WriteString(bold.Render("AgentSynch Dashboard") + "\n\n")

	if len(m.tasks) == 0 {
		b.WriteString("no tasks\n")
	} else {
		b.WriteString(bold.Render(fmt.Sprintf(
			"  %-4s  %-28s  %-11s  %-18s  %-8s  %-6s  %-10s",
			"ID", "Title", "Status", "Agent", "Heartbeat", "Tries", "Window",
		)) + "\n")
		b.WriteString("  " + strings.Repeat("─", 95) + "\n")

		for i, t := range m.tasks {
			cursor := "  "
			if i == m.cursor {
				cursor = cursorCol.Render("> ")
			}

			line := fmt.Sprintf("%s%-4d  %-28s  %s  %-18s  %-8s  %-6d  %-10s",
				cursor,
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
	claimed := byStatus(m.tasks, "claimed")
	if len(claimed) > 0 {
		b.WriteString("\n" + bold.Render("Active agents:") + "\n")
		for _, t := range claimed {
			b.WriteString(fmt.Sprintf("  task-%d: %s  agent=%s  window=%s\n",
				t.ID, t.Title,
				deref(t.ClaimedBy, "?"),
				deref(t.TmuxWindow, "none"),
			))
		}
	}

	// reaper panel
	b.WriteString("\n" + bold.Render("Reaper:") + "\n")
	if m.reapMsg == "" {
		b.WriteString("  not run yet\n")
	} else {
		b.WriteString("  " + m.reapMsg + "\n")
	}

	if m.confirming && m.cursor < len(m.tasks) {
		b.WriteString(fmt.Sprintf("\n"+errStyle.Render("delete task-%d \"%s\"? (y/n)"),
			m.tasks[m.cursor].ID, m.tasks[m.cursor].Title))
	} else {
		b.WriteString("\nj/k: navigate  a: attach to window  d: delete  q: quit")
		if m.err != "" {
			b.WriteString("  " + errStyle.Render("err: "+m.err))
		}
	}

	return b.String()
}

func colorStatus(s string) string {
	style, ok := statusColor[s]
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
