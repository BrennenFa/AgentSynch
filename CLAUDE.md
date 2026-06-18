# AgentSynch Agent Instructions
DO NOT REMOVE ANY OF MY COMMENTS UNLESS THEY ARE NOW FALSE... IN WHCIH CASE UPDATE THEM INSTEAD

ALSO DO NOT DO ANYTHING U ARE NOT INSTRUCTED TO!! 
You are a task-execution agent in the AgentSynch system.

For all CLI commands, see [`commands.md`](./commands.md).

## Your job

1. Claim the next available task
2. Check if a plan exists — if not, write one
3. Do the work described in the task
4. Mark the task `finished` (or `error` if it fails)
5. If no tasks are available, say so and stop

## Task statuses

| Status        | Meaning                                                        |
|---------------|----------------------------------------------------------------|
| `available`   | Ready to be claimed                                            |
| `blocked`     | Waiting on one or more dependency tasks to finish              |
| `claimed`     | An agent is actively working on it                             |
| `validating`  | Work submitted; waiting for a validator agent to review        |
| `finished`    | Work is done and approved                                      |
| `error`       | Task failed                                                    |
| `archived`    | Done and GH-processed (soft-delete; hidden from normal views)  |

## Step-by-step

### 1. Claim a task

```bash
cd GoCLI && go run ./cmd/... claim
```

Atomically claims the next task. The command tries `available` first (worker mode), then falls back to `validating` (validator mode). Note the task ID and mode printed.

If the output is `no available tasks`, stop.

**Worker mode output:**
```
claimed task-5: Fix login bug (agent: agent-mbp-1234)
```

**Validator mode output:**
```
claimed task-5 for validation: Fix login bug (agent: agent-mbp-5678)
```

If you claimed a task **for validation**, skip to the [Validator flow](#validator-flow) section below.

### 2. Check for a plan

If the task has a `plan`, read it and use it to guide your work.

If there is no plan, write one before executing:

```bash
cd GoCLI && go run ./cmd/... plan --id <id> --plan "your approach"
```

Keep the plan concise — what you intend to do and why.

### 3. Do the work

Execute whatever the task's `title` and `description` ask for.

### 4. Mark the task complete

On success:
```bash
cd GoCLI && go run ./cmd/... finish --id <id>
cd GoCLI && go run ./cmd/... finish --id <id> --output "optional summary"
```

On failure:
```bash
cd GoCLI && go run ./cmd/... finish --id <id> --error "what went wrong"
```

## Adding new tasks

```bash
cd GoCLI && go run ./cmd/... add --title "short task name" --description "what needs to be done"
```

Include a plan if the approach is already clear:
```bash
cd GoCLI && go run ./cmd/... add --title "..." --description "..." --plan "approach"
```

## Validator flow

When `claim` prints "for validation", you are the reviewer — not the worker. Follow these steps:

1. Read the task's `title`, `description`, `plan`, and `output` using:
   ```bash
   cd GoCLI && go run ./cmd/... list
   ```
2. Review the work described in `output` (check files, git changes, etc. mentioned there).
3. **Approve** if the work looks correct:
   ```bash
   cd GoCLI && go run ./cmd/... validate --id <id>
   ```
4. **Reject** with a specific reason if something is wrong:
   ```bash
   cd GoCLI && go run ./cmd/... validate --id <id> --reject "what was wrong"
   ```
   Rejection resets the task to `available` so an agent can redo it.

**Do NOT** write a plan. **Do NOT** use `finish`. Only use `validate`.

---

## Branch workflow

After claiming a worker task (not validation), the CLI handles branching automatically based on the hint printed by `claim`:

- **`hint: same-branch task`** — work directly on the current branch. Nothing else to do.
- **`hint: created branch task-N/...`** — the CLI already ran `git checkout -b` and recorded the branch name in the DB. Just do the work and commit.
- **`hint: create branch task-N/... and record with set-branch`** — auto-checkout failed (e.g. you have uncommitted changes). Resolve manually:

```bash
git checkout -b task-5/fix-login-bug
cd GoCLI && go run ./cmd/... set-branch --id 5 --name task-5/fix-login-bug
```

When you run `finish`, the CLI automatically pushes the branch to origin so the GitHub worker can open a PR. You do **not** need to push manually or call `set-branch` in the normal flow.

---

## Important rules

- Only claim **one task per session**. Claim it, finish it, then stop.
- Do not modify tasks claimed by other agents.
- If a task asks you to create files, create them in the project root unless the task specifies otherwise.
- If you are unsure what a task wants, make a reasonable interpretation and note it in `output`.
