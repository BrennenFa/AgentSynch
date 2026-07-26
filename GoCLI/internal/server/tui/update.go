package tui

import (
	"database/sql"
	"fmt"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"agentsynch/internal/objects"
	"agentsynch/internal/server/tui/types"
	"agentsynch/internal/store"
)

// --- reaper channel ---

type reapMsgReceived string

func waitForReap(ch chan string) tea.Cmd {
	return func() tea.Msg {
		return reapMsgReceived(<-ch)
	}
}

// --- orchestrator channel ---

type orchEventMsg types.OrchestratorEvent

func waitForOrch(ch <-chan types.OrchestratorEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil // channel closed after TUI quit
		}
		return orchEventMsg(ev)
	}
}

// --- tick + task loading ---

type tickMsg time.Time

type tasksLoadedMsg struct {
	tasks []objects.Task
	err   error
}

func tick() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func loadTasks(db *sql.DB) tea.Cmd {
	return func() tea.Msg {
		tasks, err := store.ListTasks(db)
		return tasksLoadedMsg{tasks: tasks, err: err}
	}
}

// --- BubbleTea interface ---

func (m model) Init() tea.Cmd {
	return tea.Batch(loadTasks(m.db), tick(), waitForReap(m.reapCh), waitForOrch(m.orchOutCh))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tickMsg:
		return m, tea.Batch(loadTasks(m.db), tick())

	case reapMsgReceived:
		m.reapMsg = string(msg)
		return m, tea.Batch(loadTasks(m.db), waitForReap(m.reapCh))

	case tasksLoadedMsg:
		if msg.err != nil {
			m.err = "db error: " + msg.err.Error()
		} else {
			m.tasks = msg.tasks
			m.err = ""
		}
		return m, nil

	case orchEventMsg:
		if msg.Err != "" {
			m.chatHistory = append(m.chatHistory, types.ChatEntry{From: "system", Text: "error: " + msg.Err})
		} else {
			if msg.Response != "" {
				m.chatHistory = append(m.chatHistory, types.ChatEntry{From: "claude", Text: msg.Response})
			}
			for _, s := range msg.SpawnMsgs {
				m.chatHistory = append(m.chatHistory, types.ChatEntry{From: "system", Text: s})
			}
		}
		return m, waitForOrch(m.orchOutCh)

	case tea.KeyMsg:
		// chat mode intercepts all keys except ctrl+c
		if m.chatMode {
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				m.chatMode = false
				m.chatInput = ""
			case "enter":
				if m.chatInput != "" {
					m.chatHistory = append(m.chatHistory, types.ChatEntry{From: "you", Text: m.chatInput})
					select {
					case m.orchInCh <- m.chatInput:
					default:
						m.chatHistory = append(m.chatHistory, types.ChatEntry{From: "system", Text: "[busy, try again]"})
					}
					m.chatInput = ""
				}
			case "backspace", "ctrl+h":
				if len(m.chatInput) > 0 {
					runes := []rune(m.chatInput)
					m.chatInput = string(runes[:len(runes)-1])
				}
			default:
				if len(msg.Runes) == 1 {
					m.chatInput += string(msg.Runes)
				}
			}
			return m, nil
		}

		// delete confirmation mode
		if m.confirming {
			switch msg.String() {
			case "y":
				if m.cursor < len(m.tasks) {
					id := m.tasks[m.cursor].ID
					if err := store.DeleteTask(m.db, id); err != nil {
						m.err = "delete failed: " + err.Error()
					} else {
						m.err = ""
					}
				}
				m.confirming = false
				return m, loadTasks(m.db)
			default:
				m.confirming = false
				m.err = ""
			}
			return m, nil
		}

		// normal navigation
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "j", "down":
			if m.cursor < len(m.tasks)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "a":
			if m.cursor >= len(m.tasks) {
				return m, nil
			}
			task := m.tasks[m.cursor]
			if task.TmuxWindow == nil {
				m.err = "task has no tmux window"
			} else {
				target := fmt.Sprintf("agentsynch:%s", *task.TmuxWindow)
				script := fmt.Sprintf(`tell application "Terminal" to do script "tmux attach -t %s"`, target)
				if err := exec.Command("osascript", "-e", script).Run(); err != nil {
					m.err = err.Error()
				} else {
					m.err = ""
				}
			}
		case "d":
			if m.cursor < len(m.tasks) {
				m.confirming = true
				m.err = ""
			}
		case "c":
			m.chatMode = true
			m.chatInput = ""
		}
		return m, nil
	}

	return m, nil
}
