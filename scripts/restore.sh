#!/usr/bin/env bash
# KutubCore restore (Blueprint §15). STOP THE SERVER FIRST:
#   pkill kutubcore || true
# Usage: ./scripts/restore.sh <backupDir>
set -euo pipefail
cd "$(dirname "$0")/.."
DIR="${1:-}"
if [ -z "$DIR" ]; then
  echo "usage: $0 <backupDir>" >&2
  ls -1 data/backups/ 2>/dev/null || true
  exit 1
fi
echo "Restoring $DIR (server must be stopped; current DB is safety-copied)."
exec go run ./cmd/kutubcore restore "$DIR"
