#!/bin/zsh
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
RUN_DIR="$ROOT_DIR/.run-local"

stop_one() {
  local name="$1"
  local pid_file="$2"

  if [[ ! -f "$pid_file" ]]; then
    echo "$name 未运行"
    return 0
  fi

  local pid
  pid="$(cat "$pid_file")"
  if kill -0 "$pid" >/dev/null 2>&1; then
    kill "$pid"
    echo "$name 已停止 (pid=$pid)"
  else
    echo "$name 进程已不存在 (pid=$pid)"
  fi
  rm -f "$pid_file"
}

stop_one "dashboard-lab" "$RUN_DIR/lab.pid"
stop_one "dashboard-go" "$RUN_DIR/go.pid"
