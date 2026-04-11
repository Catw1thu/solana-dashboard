#!/bin/zsh
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
RUN_DIR="$ROOT_DIR/.run-local"

print_status() {
  local name="$1"
  local pid_file="$2"

  if [[ ! -f "$pid_file" ]]; then
    echo "$name: 未运行"
    return 0
  fi

  local pid
  pid="$(cat "$pid_file")"
  if kill -0 "$pid" >/dev/null 2>&1; then
    echo "$name: 运行中 (pid=$pid)"
  else
    echo "$name: pid 文件存在但进程已退出 (pid=$pid)"
  fi
}

print_status "backend" "$RUN_DIR/backend.pid"
print_status "parser" "$RUN_DIR/parser.pid"
