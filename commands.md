# AgentSynch CLI Commands

All commands run from the `GoCLI/` directory:
```
cd GoCLI && go run ./cmd/... <command>
```

---

## tui
Open the live dashboard. Shows all tasks, runs the reaper and GitHub worker in the background.

```
go run ./cmd/... tui
```

**Keybindings:**
- `j/k` or `↑/↓` — navigate tasks
- `a` — attach to the task's tmux window in Terminal
- `d` — delete task (prompts for confirmation)
- `q` / `ctrl+c` — quit

---

## worker
Poll for available tasks and run each one in a tmux window. Requires tmux.

```
go run ./cmd/... worker
go run ./cmd/... worker --interval 10s
```

---

## add
Add a new task.

```
go run ./cmd/... add --title "short name" --description "what needs to be done"
```

---

## claim
Atomically claim the next available task.

```
go run ./cmd/... claim
```

---

## finish
Mark a claimed task as finished or error. Pushes the branch and opens a GitHub PR or issue.

```
go run ./cmd/... finish --id <id>
go run ./cmd/... finish --id <id> --output "summary of what was done"
go run ./cmd/... finish --id <id> --error "what went wrong"
```

---

## set-branch
Manually record the branch name for a claimed task. Only needed if automatic worktree creation failed.

```
go run ./cmd/... set-branch --id <id> --name <branch-name>
```
