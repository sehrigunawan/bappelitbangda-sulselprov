#!/usr/bin/env bash
# Fails if a migration that exists on the base branch was modified, deleted or renamed.
set -euo pipefail
base="${1:-origin/development}"
changed="$(git diff --name-status --diff-filter=MDR "${base}...HEAD" -- api/db/migrations/ || true)"
if [ -n "$changed" ]; then
  echo "ERROR: applied migrations must not be modified, deleted or renamed. Add a new migration instead."
  echo "$changed"
  exit 1
fi
echo "migrations: OK (only additions)"
