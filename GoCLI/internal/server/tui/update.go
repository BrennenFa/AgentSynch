package tui

import (
	"database/sql"
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"agentsynch/internal/objects"
	"agentsynch/internal/store"
	"agentsynch/internal/worker"
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

type previewLoadedMsg string

type spawnResultMsg struct{ err error }

func spawnAgentCmd(db *sql.DB) tea.Cmd {
	return func() tea.Msg {
		return spawnResultMsg{err: worker.SpawnAgent(db)}
	}
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

// loadPreview captures the tmux pane for the currently selected task.
// Returns previewLoadedMsg with the output, or empty string if no window.
func loadPreview(tasks []objects.Task, cursor int) tea.Cmd {
	return func() tea.Msg {
		if cursor < 0 || cursor >= len(tasks) {
			return previewLoadedMsg("")
		}
		t := tasks[cursor]
		if t.TmuxWindow == nil {
			return previewLoadedMsg("")
		}
		target := fmt.Sprintf("agentsynch:%s", *t.TmuxWindow)
		out, err := exec.Command("tmux", "capture-pane", "-t", target, "-p").Output()
		if err != nil {
			return previewLoadedMsg("")
		}
		// strip trailing blank lines
		lines := strings.Split(string(out), "\n")
		last := len(lines)
		for last > 0 && strings.TrimSpace(lines[last-1]) == "" {
			last--
		}
		return previewLoadedMsg(strings.Join(lines[:last], "\n"))
	}
}

func (m model) activePreviewCmd() tea.Cmd {
	if m.activeTab == 0 {
		return loadPreview(m.tasks, m.cursor)
	}
	return loadPreview(byStatus(m.tasks, "claimed"), m.agentCursor)
}

func (m model) Init() tea.Cmd {
	return tea.Batch(loadTasks(m.db), tick(), waitForReap(m.reapCh))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case previewLoadedMsg:
		m.preview = string(msg)
		return m, nil

	case tickMsg:
		return m, tea.Batch(loadTasks(m.db), tick(), m.activePreviewCmd())

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
		return m, m.activePreviewCmd()

	case spawnResultMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
		}
		return m, loadTasks(m.db)

	case tea.KeyMsg:
		// ── kill-agent confirmation ───────────────────────────────────────────
		if m.confirmingKill {
			agents := byStatus(m.tasks, "claimed")
			switch msg.String() {
			case "y":
				if m.agentCursor < len(agents) {
					agent := agents[m.agentCursor]
					if agent.TmuxWindow != nil {
						target := fmt.Sprintf("agentsynch:%s", *agent.TmuxWindow)
						exec.Command("tmux", "kill-window", "-t", target).Run() //nolint:errcheck
					}
					if err := store.ResetTask(m.db, agent.ID); err != nil {
						m.err = "reset failed: " + err.Error()
					} else {
						m.err = ""
					}
				}
				m.confirmingKill = false
				return m, loadTasks(m.db)
			default:
				m.confirmingKill = false
				m.err = ""
			}
			return m, nil
		}

		// ── delete confirmation ───────────────────────────────────────────────
		if m.confirming {
			switch msg.String() {
			case "y":
				if m.cursor < len(m.tasks) {
					task := m.tasks[m.cursor]
					if task.TmuxWindow != nil {
						target := fmt.Sprintf("agentsynch:%s", *task.TmuxWindow)
						exec.Command("tmux", "kill-window", "-t", target).Run() //nolint:errcheck
					}
					if err := store.DeleteTask(m.db, task.ID); err != nil {
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

		// ── normal key handling ───────────────────────────────────────────────
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "tab":
			m.activeTab = (m.activeTab + 1) % 2
			m.preview = ""
			return m, m.activePreviewCmd()

		case "j", "down":
			if m.activeTab == 0 {
				if m.cursor < len(m.tasks)-1 {
					m.cursor++
					m.preview = ""
					return m, loadPreview(m.tasks, m.cursor)
				}
			} else {
				agents := byStatus(m.tasks, "claimed")
				if m.agentCursor < len(agents)-1 {
					m.agentCursor++
					m.preview = ""
					return m, loadPreview(agents, m.agentCursor)
				}
			}

		case "k", "up":
			if m.activeTab == 0 {
				if m.cursor > 0 {
					m.cursor--
					m.preview = ""
					return m, loadPreview(m.tasks, m.cursor)
				}
			} else {
				if m.agentCursor > 0 {
					m.agentCursor--
					agents := byStatus(m.tasks, "claimed")
					m.preview = ""
					return m, loadPreview(agents, m.agentCursor)
				}
			}

		case "n":
			// spawn agent — agents tab only
			if m.activeTab != 1 {
				return m, nil
			}
			if len(byStatus(m.tasks, "available")) == 0 {
				m.err = "no available tasks"
				return m, nil
			}
			return m, spawnAgentCmd(m.db)

		case "o":
			// open tmux session in a new Terminal window (once; reuses if already open)
			script := `tell application "Terminal" to do script "tmux attach -t agentsynch"`
			if err := exec.Command("osascript", "-e", script).Run(); err != nil {
				m.err = err.Error()
			} else {
				m.err = ""
			}

		case "a":
			// switch the active window in the already-open tmux terminal
			var task objects.Task
			var found bool
			if m.activeTab == 0 {
				if m.cursor < len(m.tasks) {
					task = m.tasks[m.cursor]
					found = true
				}
			} else {
				agents := byStatus(m.tasks, "claimed")
				if m.agentCursor < len(agents) {
					task = agents[m.agentCursor]
					found = true
				}
			}
			if !found {
				return m, nil
			}
			if task.TmuxWindow == nil {
				m.err = "no tmux window for this task"
			} else {
				if err := exec.Command("tmux", "select-window", "-t", fmt.Sprintf("agentsynch:%s", *task.TmuxWindow)).Run(); err != nil {
					m.err = err.Error()
				} else {
					m.err = ""
				}
			}

		case "d":
			// delete — tasks tab only
			if m.activeTab != 0 {
				return m, nil
			}
			if m.cursor < len(m.tasks) {
				m.confirming = true
				m.err = ""
			}

		case "x":
			// kill agent — agents tab only
			if m.activeTab != 1 {
				return m, nil
			}
			agents := byStatus(m.tasks, "claimed")
			if m.agentCursor < len(agents) {
				m.confirmingKill = true
				m.err = ""
			}

		case "p":
			// open task note in Obsidian — tasks tab only
			if m.activeTab != 0 {
				return m, nil
			}
			if m.vaultPath == "" {
				m.err = "no vault configured"
				return m, nil
			}
			if m.cursor >= len(m.tasks) {
				return m, nil
			}
			task := m.tasks[m.cursor]
			vaultName := filepath.Base(m.vaultPath)
			relFile := filepath.Join("AgentSynch", m.repoName, "tasks", fmt.Sprintf("task-%d.md", task.ID))
			obsidianURL := fmt.Sprintf("obsidian://open?vault=%s&file=%s", url.QueryEscape(vaultName), url.QueryEscape(relFile))
			if err := exec.Command("open", obsidianURL).Run(); err != nil {
				m.err = err.Error()
			} else {
				m.err = ""
			}
		}
		return m, nil
	}

	return m, nil
}
