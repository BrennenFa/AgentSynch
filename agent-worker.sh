#!/bin/bash
INTERVAL=${1:-10}
BINARY="GoCLI/agentsynch"   # use compiled binary, not go run

echo "Agent waiting for tasks (polling every ${INTERVAL}s)..."

while true; do
  result=$(./$BINARY claim)
  if [[ "$result" != "no available tasks" ]]; then
    echo "Task found: $result"
    claude "$result — follow CLAUDE.md to complete this task."
    break
  fi
  sleep "$INTERVAL"
done
