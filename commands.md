# AgentSynch CLI Commands

All commands run from the `GoCLI/` directory:
```
cd GoCLI && go run ./cmd/... <command>
```

---

## claim
Atomically claim the next task. Run this first.

```
go run ./cmd/... claim
```

Tries to claim the first `available` task (worker mode). If none exist, falls back to the first `validating` task with no assigned validator (validator mode). Prints the task ID, title, and your agent ID. If both queues are empty, prints `no available tasks`.

**Worker mode** — you execute the task:
```
claimed task-5: Fix login bug (agent: agent-mbp-1234)
```

**Validator mode** — you review the work and call `validate`:
```
claimed task-5 for validation: Fix login bug (agent: agent-mbp-5678)
```

---

## plan
Write a plan for a claimed task. Required before moving to `in_progress` if no plan exists.

```
go run ./cmd/... plan --id <id> --plan "your approach"
```

---

## finish
Mark a claimed task as finished or error.

```
go run ./cmd/... finish --id <id>
go run ./cmd/... finish --id <id> --output "optional summary"
go run ./cmd/... finish --id <id> --error "what went wrong"
```

---

## add
Add a new task. `--plan` is optional — if provided the agent will skip planning and execute directly. Use `--same-branch` for trivial tasks that don't need a new git branch.

```
go run ./cmd/... add --title "short name" --description "what needs to be done"
go run ./cmd/... add --title "short name" --description "what needs to be done" --plan "approach"
go run ./cmd/... add --title "short name" --description "what needs to be done" --same-branch
```

---

## list
List all tasks and their current status. Archived tasks are hidden by default; use `--all` to include them.

```
go run ./cmd/... list
go run ./cmd/... list --all
```

---

## set-branch
Record the git branch created for a claimed task. Call this after `git checkout -b <branch>` and before `finish`. Enables the server to open a GitHub PR on completion.

```
go run ./cmd/... set-branch --id <id> --name <branch-name>
```

---

## archive
Manually archive a finished or error task. Normally done automatically by the server after GH automation runs.

```
go run ./cmd/... archive --id <id>
```

---

## validate
Approve or reject a task that is in `validating` status. Only used when you claimed a task **for validation**.

```
go run ./cmd/... validate --id <id>
go run ./cmd/... validate --id <id> --reject "reason the work needs to be redone"
```

Approving moves the task to `finished` and unblocks any dependents. Rejecting resets it to `available` with an error note so an agent can redo it.

---

## Statuses

| Status       | Meaning                                                    |
|--------------|------------------------------------------------------------|
| `available`  | Ready to be claimed                                        |
| `blocked`    | Waiting on one or more dependency tasks to finish          |
| `claimed`    | An agent is actively working on it                         |
| `validating` | Work submitted; waiting for a validator agent to review    |
| `finished`   | Work is complete and approved                              |
| `error`      | Task failed                                                |
| `archived`   | Done and GH-processed; hidden from normal views (`list`, TUI) |
