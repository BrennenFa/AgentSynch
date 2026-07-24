package tui

import (
	"database/sql"
	"fmt"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"agentsynch/internal/objects"
	"agentsynch/internal/store"
)

type reapMsgReceived string

func waitForReap(ch chan string) tea.Cmd {
	return func() tea.Msg {
		return reapMsgReceived(<-ch)
	}
}

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

func (m model) Init() tea.Cmd {
	return tea.Batch(loadTasks(m.db), tick(), waitForReap(m.reapCh))
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

	case tea.KeyMsg:
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
		}
		return m, nil
	}

	return m, nil
}
