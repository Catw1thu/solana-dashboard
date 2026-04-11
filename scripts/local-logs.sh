#!/bin/zsh
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
LOG_DIR="$ROOT_DIR/logs-local"

mkdir -p "$LOG_DIR"

touch "$LOG_DIR/go.log" "$LOG_DIR/lab.log"

tail -n 200 -f "$LOG_DIR/go.log" "$LOG_DIR/lab.log"
