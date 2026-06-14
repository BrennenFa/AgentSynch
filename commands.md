# AgentSynch CLI Commands

All commands run from the `GoCLI/` directory:
```
cd GoCLI && go run ./cmd/... <command>
```

---

## claim
Atomically claim the next available task. Run this first.

```
go run ./cmd/... claim
```

Finds the first `available` task, marks it `claimed`, and prints the task ID, title, and your agent ID. If no tasks are available, prints `no available tasks`.

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
Add a new task. `--plan` is optional — if provided the agent will skip planning and execute directly.

```
go run ./cmd/... add --title "short name" --description "what needs to be done"
go run ./cmd/... add --title "short name" --description "what needs to be done" --plan "approach"
```

---

## list
List all tasks and their current status.

```
go run ./cmd/... list
```

---

## Statuses

| Status      | Meaning                              |
|-------------|--------------------------------------|
| `available` | Ready to be claimed                  |
| `claimed`   | An agent is actively working on it   |
| `finished`  | Work is complete                     |
| `error`     | Task failed                          |
