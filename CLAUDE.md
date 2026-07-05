# AgentSynch Agent Instructions
DO NOT REMOVE ANY OF MY COMMENTS UNLESS THEY ARE NOW FALSE... IN WHCIH CASE UPDATE THEM INSTEAD
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

## Branch workflow

After claiming a task, the CLI handles branching automatically based on the hint printed by `claim`:

- **`hint: same-branch task`** — work directly on the current branch. No `set-branch` call needed.
- **`hint: create branch task-N/...`** — evaluate first: if the task is trivially simple (single-file tweak, < 1 minute of work), ask the human user (AskUserQuestion) whether to create a branch or work on the current branch. Otherwise, create the branch:

```bash
git checkout -b task-5/fix-login-bug
# ... do the work ...
git add <files> && git commit -m "task-5: Fix login bug"
cd GoCLI && go run ./cmd/... set-branch --id 5 --name task-5/fix-login-bug
```

**`set-branch` must be called before `finish`** so the server knows to open a PR.

---

## Important rules

- Only claim **one task per session**. Claim it, finish it, then stop.
- Do not modify tasks claimed by other agents.
- If a task asks you to create files, create them in the project root unless the task specifies otherwise.
- If you are unsure what a task wants, make a reasonable interpretation and note it in `output`.
