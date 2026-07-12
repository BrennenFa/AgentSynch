package tui

import (
	"database/sql"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"agentsynch/internal/objects"
	"agentsynch/internal/store"
	"agentsynch/internal/worker"
)

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
	return tea.Batch(loadTasks(m.db), tick())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tickMsg:
		return m, tea.Batch(loadTasks(m.db), tick())

	case tasksLoadedMsg:
		if msg.err != nil {
			m.err = "db error: " + msg.err.Error()
		} else {
			m.tasks = msg.tasks
			m.err = ""
		}
		return m, nil

	case tea.KeyMsg:
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
			} else if os.Getenv("TMUX") == "" {
				m.err = "not in tmux — run: tmux attach -t agentsynch"
			} else if err := worker.SwitchClient(*task.TmuxWindow); err != nil {
				m.err = err.Error()
			} else {
				m.err = ""
			}
		}
		return m, nil
	}

	return m, nil
}
