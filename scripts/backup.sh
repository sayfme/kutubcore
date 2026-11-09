#!/usr/bin/env bash
# KutubCore backup (Blueprint §15). Server may stay running for backup,
# but must be stopped for restore (scripts/restore.sh).
# Usage: ./scripts/backup.sh [destDir]   (default: data/backups)
set -euo pipefail
cd "$(dirname "$0")/.."
DEST="${1:-data/backups}"
mkdir -p "$DEST"
exec go run ./cmd/kutubcore backup --dest "$DEST"
