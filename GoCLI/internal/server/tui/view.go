package tui

import (
	"fmt"

	"agentsynch/internal/server/tui/types"
	"agentsynch/internal/server/tui/ui"
)

func (m model) View() string {
	top := ui.RenderTaskTable(m.tasks, m.cursor, m.confirming, m.reapMsg, m.err)

	// overlay the delete-confirm prompt when confirming
	if m.confirming && m.cursor < len(m.tasks) {
		top += fmt.Sprintf("\n"+ui.ErrStyle.Render("delete task-%d \"%s\"? (y/n)"),
			m.tasks[m.cursor].ID, m.tasks[m.cursor].Title)
	}

	bottom := ui.RenderChat(types.ChatState{
		Mode:    m.chatMode,
		Input:   m.chatInput,
		History: m.chatHistory,
	})

	return top + bottom
}
