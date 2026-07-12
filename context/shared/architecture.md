# AgentSynch Architecture

## Overview
AgentSynch is a multi-agent task orchestration system. Agents claim tasks from a shared SQLite database, do work on isolated git worktrees, and push branches for PR review.

## Components

### GoCLI
The core CLI written in Go. Agents interact with the system exclusively through this binary.

- `cmd/` — entrypoint, routes subcommands
- `internal/commands/` — one file per subcommand (claim, finish, add, list, etc.)
- `internal/store/` — all SQLite read/write logic
- `internal/objects/` — shared data types (Task, etc.)
- `internal/server/` — background server: reaper, GitHub automation, TUI

### Database
SQLite at `~/.agentsynch/tasks.db`. Schema lives in `store/db.go`. Migrations are additive ALTER TABLE statements that ignore "duplicate column name" errors so the DB can be upgraded in place.

### Worktrees
On claim, the CLI creates a git worktree sibling directory (`AgentSynch-task-N/slug`) and checks out a new branch there. Agents do all work in that directory, then `finish` pushes the branch.

### Context System
Markdown files at `context/` in the project root. Agents read and write these for shared memory.
- `context/tasks/task-{id}.md` — per-task notes
- `context/shared/` — system-wide reference docs (this directory)

The `context_docs` SQLite table catalogs shared docs. The `shared_docs` column on tasks links relevant shared doc names (JSON array).
