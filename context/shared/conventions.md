# AgentSynch Conventions

## CLI commands
All commands run from `GoCLI/` via `go run ./cmd/... <command>`. See `commands.md` for full reference.

## Git workflow
- Each task gets its own worktree at `AgentSynch-task-N/slug`
- Never commit directly to main
- `finish` auto-pushes the branch; no manual push needed

## Task rules
- Claim one task per session only
- Do not touch tasks claimed by other agents
- Write a plan before executing if none exists

## Code conventions
- Go only for CLI/store/server logic
- Migrations: additive ALTER TABLE in `store/db.go`, ignore duplicate column errors
- All DB reads go through `store/tasks.go` scanTask + allColumns
- New columns: add to schema or migrations, Task struct, allColumns, scanTask — in that order

## Context files
- Write per-task notes to `context/tasks/task-{id}.md` as you work
- Update shared docs in `context/shared/` if something system-wide changes
- Keep notes factual — what you did, what you found, any gotchas
