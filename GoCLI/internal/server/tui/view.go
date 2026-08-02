package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"agentsynch/internal/objects"
)

var (
	bold            = lipgloss.NewStyle().Bold(true)
	cursorCol       = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	errStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	activeTabStyle  = lipgloss.NewStyle().Bold(true).Underline(true)
	inactiveTabStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	statusColor = map[string]lipgloss.Style{
		"available": lipgloss.NewStyle().Foreground(lipgloss.Color("2")),
		"claimed":   lipgloss.NewStyle().Foreground(lipgloss.Color("3")),
		"finished":  lipgloss.NewStyle().Foreground(lipgloss.Color("4")),
		"error":     lipgloss.NewStyle().Foreground(lipgloss.Color("1")),
		"archived":  lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
	}
)

func (m model) View() string {
	// default dimensions before first WindowSizeMsg
	w := m.width
	h := m.height
	if w == 0 {
		w = 160
	}
	if h == 0 {
		h = 40
	}

	leftW := w * 2 / 5
	rightW := w - leftW - 1 // -1 for the right border of the left pane
	contentH := h - 4       // header 2 lines + footer 2 lines

	// ── left pane ────────────────────────────────────────────────────────────
	var leftB strings.Builder

	if m.activeTab == 0 {
		// tasks tab: ID(4)  Status(11)  Title(remaining)
		titleW := leftW - 4 - 11 - 4 // subtract ID, status, spacing
		if titleW < 8 {
			titleW = 8
		}

		leftB.WriteString(bold.Render(fmt.Sprintf("%-4s  %-11s  %s", "ID", "Status", "Title")) + "\n")
		leftB.WriteString(strings.Repeat("─", leftW) + "\n")

		for i, t := range m.tasks {
			cursor := "  "
			if i == m.cursor {
				cursor = cursorCol.Render("> ")
			}
			line := fmt.Sprintf("%s%-4d  %s  %s",
				cursor,
				t.ID,
				colorStatus(t.Status),
				trunc(t.Title, titleW),
			)
			leftB.WriteString(line + "\n")
		}
	} else {
		// agents tab: Agent(20)  Task(remaining)  Beat(8)
		agents := byStatus(m.tasks, "claimed")
		agentColW := 20
		beatColW := 8
		taskColW := leftW - agentColW - beatColW - 4 - 2 // 4 spacing, 2 cursor
		if taskColW < 8 {
			taskColW = 8
		}

		leftB.WriteString(bold.Render(fmt.Sprintf("  %-20s  %-*s  %-8s", "Agent", taskColW, "Task", "Beat")) + "\n")
		leftB.WriteString(strings.Repeat("─", leftW) + "\n")

		for i, t := range agents {
			cursor := "  "
			if i == m.agentCursor {
				cursor = cursorCol.Render("> ")
			}
			line := fmt.Sprintf("%s%-20s  %-*s  %s",
				cursor,
				trunc(deref(t.ClaimedBy, "?"), agentColW),
				taskColW,
				trunc(t.Title, taskColW),
				heartbeatStr(t.HeartbeatAt),
			)
			leftB.WriteString(line + "\n")
		}
	}

	leftPane := lipgloss.NewStyle().
		Width(leftW).
		Height(contentH).
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(lipgloss.Color("8")).
		Render(leftB.String())

	// ── right pane ───────────────────────────────────────────────────────────
	var rightB strings.Builder

	if m.activeTab == 0 {
		if m.addingTask {
			// add-task form
			focused := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
			dim := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

			rightB.WriteString(bold.Render("Add Task") + "\n")
			rightB.WriteString(strings.Repeat("─", rightW-2) + "\n\n")

			if m.formField == 0 {
				rightB.WriteString(focused.Render("Title:") + "\n")
				rightB.WriteString(focused.Render("> "+m.titleInput+"▌") + "\n\n")
			} else {
				rightB.WriteString(bold.Render("Title:") + "\n")
				rightB.WriteString(dim.Render("> "+m.titleInput) + "\n\n")
			}

			if m.formField == 1 {
				rightB.WriteString(focused.Render("Description:") + "\n")
				rightB.WriteString(focused.Render("> "+m.descInput+"▌") + "\n\n")
			} else {
				rightB.WriteString(bold.Render("Description:") + "\n")
				rightB.WriteString(dim.Render("> "+m.descInput) + "\n\n")
			}
		} else if len(m.tasks) == 0 {
			rightB.WriteString("no tasks")
		} else if m.cursor < len(m.tasks) {
			t := m.tasks[m.cursor]

			rightB.WriteString(bold.Render(fmt.Sprintf("task-%d: %s", t.ID, t.Title)) + "\n")
			rightB.WriteString(strings.Repeat("─", rightW-2) + "\n")
			rightB.WriteString(fmt.Sprintf("%-8s %s\n", "agent:", deref(t.ClaimedBy, "-")))
			rightB.WriteString(fmt.Sprintf("%-8s %s\n", "window:", deref(t.TmuxWindow, "-")))
			rightB.WriteString(fmt.Sprintf("%-8s %s\n", "beat:", heartbeatStr(t.HeartbeatAt)))
			rightB.WriteString(fmt.Sprintf("%-8s %s\n", "status:", colorStatus(t.Status)))
			rightB.WriteString("\n")

			rightB.WriteString("── terminal " + strings.Repeat("─", rightW-14) + "\n")

			// previewH = contentH - (title + sep + 4 fields + blank + terminal header)
			previewH := contentH - 8
			if previewH < 1 {
				previewH = 1
			}

			if t.TmuxWindow == nil {
				rightB.WriteString("(no active session)")
			} else if m.preview == "" {
				rightB.WriteString("loading...")
			} else {
				lines := strings.Split(m.preview, "\n")
				// take last previewH lines
				if len(lines) > previewH {
					lines = lines[len(lines)-previewH:]
				}
				for _, l := range lines {
					if len(l) > rightW-1 {
						l = l[:rightW-1]
					}
					rightB.WriteString(l + "\n")
				}
			}
		}
	} else {
		// agents tab right pane
		agents := byStatus(m.tasks, "claimed")
		if len(agents) == 0 {
			rightB.WriteString("no active agents")
		} else if m.agentCursor < len(agents) {
			t := agents[m.agentCursor]
			agentName := deref(t.ClaimedBy, "?")

			rightB.WriteString(bold.Render(agentName) + "\n")
			rightB.WriteString(strings.Repeat("─", rightW-2) + "\n")
			rightB.WriteString(fmt.Sprintf("%-8s %s\n", "task:", t.Title))
			rightB.WriteString(fmt.Sprintf("%-8s %s\n", "window:", deref(t.TmuxWindow, "-")))
			rightB.WriteString(fmt.Sprintf("%-8s %s\n", "beat:", heartbeatStr(t.HeartbeatAt)))
			rightB.WriteString(fmt.Sprintf("%-8s %s\n", "status:", colorStatus(t.Status)))
			rightB.WriteString("\n")

			rightB.WriteString("── terminal " + strings.Repeat("─", rightW-14) + "\n")

			previewH := contentH - 8
			if previewH < 1 {
				previewH = 1
			}

			if t.TmuxWindow == nil {
				rightB.WriteString("(no active session)")
			} else if m.preview == "" {
				rightB.WriteString("loading...")
			} else {
				lines := strings.Split(m.preview, "\n")
				if len(lines) > previewH {
					lines = lines[len(lines)-previewH:]
				}
				for _, l := range lines {
					if len(l) > rightW-1 {
						l = l[:rightW-1]
					}
					rightB.WriteString(l + "\n")
				}
			}
		}
	}

	rightPane := lipgloss.NewStyle().
		Width(rightW).
		Height(contentH).
		PaddingLeft(1).
		Render(rightB.String())

	// ── assemble ─────────────────────────────────────────────────────────────
	var out strings.Builder

	// header with tab labels
	tasksLabel := "[Tasks]"
	agentsLabel := "[Agents]"
	if m.activeTab == 0 {
		tasksLabel = activeTabStyle.Render(tasksLabel)
		agentsLabel = inactiveTabStyle.Render(agentsLabel)
	} else {
		tasksLabel = inactiveTabStyle.Render(tasksLabel)
		agentsLabel = activeTabStyle.Render(agentsLabel)
	}
	out.WriteString("AgentSynch  " + tasksLabel + "  " + agentsLabel + "\n\n")

	out.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane))
	out.WriteString("\n")

	// footer
	if m.confirming && m.cursor < len(m.tasks) {
		out.WriteString(errStyle.Render(fmt.Sprintf("delete task-%d \"%s\"? (y/n)",
			m.tasks[m.cursor].ID, m.tasks[m.cursor].Title)))
	} else if m.confirmingKill {
		agents := byStatus(m.tasks, "claimed")
		if m.agentCursor < len(agents) {
			agentName := deref(agents[m.agentCursor].ClaimedBy, "?")
			out.WriteString(errStyle.Render(fmt.Sprintf("kill agent \"%s\" and reset task? (y/n)", agentName)))
		}
	} else if m.addingTask {
		hint := inactiveTabStyle.Render("tab: switch field  enter: submit  esc: cancel")
		if m.err != "" {
			hint += "  " + errStyle.Render("err: "+m.err)
		}
		out.WriteString(hint)
	} else {
		var footer string
		reapInfo := ""
		if m.reapMsg != "" {
			reapInfo = "  reaper: " + m.reapMsg
		}
		if m.activeTab == 0 {
			footer = "tab: switch  j/k: navigate  o: open tmux  a: select window  p: open in obsidian  c: add  d: delete  q: quit" + reapInfo
		} else {
			footer = "tab: switch  j/k: navigate  n: spawn agent  o: open tmux  a: select window  x: kill  q: quit" + reapInfo
		}
		if m.err != "" {
			footer += "  " + errStyle.Render("err: "+m.err)
		}
		out.WriteString(footer)
	}

	return out.String()
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
