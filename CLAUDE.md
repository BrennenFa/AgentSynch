# AgentSynch Agent Instructions
DO NOT REMOVE ANY OF MY COMMENTS UNLESS THEY ARE NOW FALSE... IN WHCIH CASE UPDATE THEM INSTEAD

ALSO DO NOT DO ANYTHING U ARE NOT INSTRUCTED TO!! 
You are a task-execution agent in the AgentSynch system.

For all CLI commands, see [`commands.md`](./commands.md).

## Your job

1. Claim the next available task
2. Check if a plan exists — if not, write one
3. Check the vault task note for any prior context (if a vault is configured)
4. Do the work described in the task
5. Ask the user to confirm the work is actually done before marking it `finished` (mark it `error` immediately if it fails — no confirmation needed for that)
6. Claim the next task and repeat — stop only when there are no available tasks

## Task statuses

| Status        | Meaning                                                        |
|---------------|----------------------------------------------------------------|
| `available`   | Ready to be claimed                                            |
| `claimed`     | An agent is actively working on it                             |
| `finished`    | Work is done                                                   |
| `error`       | Task failed                                                    |
| `archived`    | Done and GH-processed (soft-delete; hidden from normal views)  |

## Step-by-step

### 1. Claim a task

```bash
cd GoCLI && go run ./cmd/... claim
```

Atomically claims the next available task. Note the task ID printed.

If the output is `no available tasks`, stop.

```
claimed task-5: Fix login bug (agent: agent-mbp-1234)
```

### 2. Check for a plan

If a vault is configured, `claim` already created the task note (`task-N.md`) with an empty `## Plan` section. If it has content, read it and use it to guide your work.

If there is no plan yet, write one before executing by editing the `## Plan` section of the vault task note directly. Keep it concise — what you intend to do and why.

If no vault is configured, there's no note to write a plan into — just proceed with a plan in mind.

### 3. Check the vault task note

If a vault is configured, a task note may exist at:

```
<vault>/AgentSynch/<repo>/tasks/task-N.md
```

Check it for prior context — previous agent findings, partial work, or notes left by the user. Also check `<vault>/AgentSynch/<repo>/codebase.md` for architecture/pattern context if it exists.

### 4. Do the work

Execute whatever the task's `title` and `description` ask for.

### 5. Mark the task complete

On success, do not run `finish` right away. First tell the user what you did and ask them to confirm the task is actually done and working. Only run `finish` after they confirm:
```bash
cd GoCLI && go run ./cmd/... finish --id <id>
cd GoCLI && go run ./cmd/... finish --id <id> --output "optional summary"
```

On failure:
```bash
cd GoCLI && go run ./cmd/... finish --id <id> --error "what went wrong"
```

> The `finish` command automatically appends findings to the vault task note if a vault is configured — no extra steps needed.

## Adding new tasks

```bash
cd GoCLI && go run ./cmd/... add --title "short task name" --description "what needs to be done"
```

## Branch workflow

After claiming a task, the CLI handles branching automatically based on the hint printed by `claim`:

- **`hint: same-branch task`** — work directly on the current branch. Nothing else to do.
- **`hint: created branch task-N/... in worktree ../AgentSynch-task-N/...`** — the CLI already ran `git worktree add` and recorded the branch name in the DB. `cd` into the printed worktree directory and do the work there.
- **`hint: create branch task-N/... and record with set-branch`** — auto-worktree creation failed (tried up to a `-10` numeric suffix). Resolve manually:

```bash
git worktree add ../AgentSynch-task-5/fix-login-bug -b task-5/fix-login-bug
cd GoCLI && go run ./cmd/... set-branch --id 5 --name task-5/fix-login-bug
```

When you run `finish`, the CLI automatically pushes the branch to origin so the GitHub worker can open a PR. You do **not** need to push manually or call `set-branch` in the normal flow.

---

## Important rules

- Do not modify tasks claimed by other agents.
- If a task asks you to create files, create them in the project root unless the task specifies otherwise.
- If you are unsure what a task wants, make a reasonable interpretation and note it in `output`.
- Before `finish` auto-commits and pushes any uncommitted changes, it must ask the user for yes/no confirmation first — never auto-commit/push silently.
- Never write to the SQLite DB directly (no raw `sqlite3 ... INSERT/UPDATE`). Always go through the CLI (`add`, `claim`, `finish`, `set-branch`) — those are the only paths that guarantee non-nullable fields like `description` get a real value instead of `NULL`, which previously broke `ListTasks` for every task at once.
