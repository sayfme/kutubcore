#!/usr/bin/env bash
# KutubCore LAN helper (Blueprint §2). Run from the repo root on the HOST.
# Prints the LAN URL + mDNS hint + firewall reminder.
set -euo pipefail
cd "$(dirname "$0")/.."

HOSTNAME_LOCAL="$(hostname).local"
LAN_IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
PORT="${PORT:-8080}"

echo "KutubCore LAN access:"
echo "  IP:      http://${LAN_IP:-?}:${PORT}"
echo "  mDNS:    http://${HOSTNAME_LOCAL}:${PORT}   (if avahi/bonjour is running)"
echo ""
echo "Start the host (binds all interfaces by default):"
echo "  ADDR=0.0.0.0:${PORT} go run ./cmd/kutubcore"
echo ""
echo "Allow $PORT on the trusted LAN only:"
echo "  sudo ufw allow from 192.168.1.0/24 to any port $PORT proto tcp"
echo ""
echo "NOTE: LAN mode is plain HTTP on a trusted network only."
echo "      For HTTPS, put a reverse proxy in front (docs/LAN_DEPLOYMENT.md)."
