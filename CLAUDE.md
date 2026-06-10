# AgentSynch Agent Instructions

You are a task-execution agent in the AgentSynch system.

## Your job

1. Read `tasks.yaml` and find a task with `status: available`
2. Claim it by setting `status: claimed` and `claimed_by` to a unique agent ID (use format `agent-<timestamp>`, e.g. `agent-1749520620`)
3. Do the work described in the task
4. Mark the task `finished` (or `error` if it fails)
5. If no tasks are available, say so and stop

## Task statuses

| Status      | Meaning                                      |
|-------------|----------------------------------------------|
| `available` | Ready to be claimed                          |
| `claimed`   | An agent is actively working on it           |
| `finished`  | Work is done, output recorded                |
| `error`     | Task failed, error message recorded          |

## Step-by-step

### 1. Find an available task

Read `tasks.yaml`. Look for the first task where `status: available`.

If none exist, output: "No available tasks. Exiting." and stop.

### 2. Claim the task

Edit `tasks.yaml` to update that task:
```yaml
status: claimed
claimed_by: agent-<unix_timestamp>   # e.g. agent-1749520620
claimed_at: "<ISO timestamp>"
```

Use the current Unix timestamp as your agent ID so it is unique across concurrent agents.

### 3. Do the work

Read the task's `title` and `description`. Execute whatever is asked.
- Write files, run bash commands, produce output — whatever the task requires.
- Record a summary of what you did in the task's `output` field.

### 4. Mark the task complete

On success, update the task in `tasks.yaml`:
```yaml
status: finished
finished_at: "<ISO timestamp>"
output: "<brief summary of what was done>"
```

On failure, update:
```yaml
status: error
finished_at: "<ISO timestamp>"
error: "<description of what went wrong>"
```

## Important rules

- Only claim **one task per session**. Claim it, finish it, then stop.
- Do not modify tasks claimed by other agents.
- Write your output directly into the `output` field in `tasks.yaml` — keep it short (1–3 sentences).
- If a task asks you to create files, create them in the project root unless the task specifies otherwise.
- If you are unsure what a task wants, make a reasonable interpretation and note it in `output`.

## Quick reference: editing tasks.yaml

Use the Edit tool to update the specific fields of the task you claimed. Do not rewrite the whole file.
