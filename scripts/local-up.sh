#!/bin/zsh
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
ENV_FILE="$ROOT_DIR/.env"
RUN_DIR="$ROOT_DIR/.run-local"
LOG_DIR="$ROOT_DIR/logs-local"
BACKEND_DIR="$ROOT_DIR/backend"
PARSER_DIR="$ROOT_DIR/parser"
BACKEND_BIN="$BACKEND_DIR/bin/dashboard-api"
PARSER_BIN="$PARSER_DIR/target/release/solana-dashboard-lab"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "缺少 $ENV_FILE"
  echo "先执行：cp .env.example .env"
  exit 1
fi

source "$ENV_FILE"

: "${DATABASE_URL:?DATABASE_URL is required}"
: "${REDIS_URL:?REDIS_URL is required}"
: "${NATS_URL:?NATS_URL is required}"
: "${GRPC_ENDPOINT:?GRPC_ENDPOINT is required}"
: "${GRPC_TOKEN:?GRPC_TOKEN is required}"

mkdir -p "$RUN_DIR" "$LOG_DIR" "$BACKEND_DIR/bin"

start_if_needed() {
  local name="$1"
  local pid_file="$2"
  shift 2

  if [[ -f "$pid_file" ]]; then
    local pid
    pid="$(cat "$pid_file")"
    if kill -0 "$pid" >/dev/null 2>&1; then
      echo "$name 已在运行 (pid=$pid)"
      return 0
    fi
    rm -f "$pid_file"
  fi

  "$@" &
  local pid=$!
  echo "$pid" > "$pid_file"
  echo "$name 已启动 (pid=$pid)"
}

if [[ ! -x "$BACKEND_BIN" ]]; then
  echo "构建 Go 二进制..."
  (
    cd "$BACKEND_DIR"
    go build -o "$BACKEND_BIN" ./cmd/api
  )
fi

if [[ ! -x "$PARSER_BIN" ]]; then
  echo "构建 Rust release 二进制..."
  (
    cd "$PARSER_DIR"
    cargo build --release
  )
fi

start_if_needed "backend" "$RUN_DIR/backend.pid" \
  env DATABASE_URL="$DATABASE_URL" API_ADDR="${API_ADDR:-:8081}" NATS_URL="$NATS_URL" \
  nohup "$BACKEND_BIN" >> "$LOG_DIR/backend.log" 2>&1

start_if_needed "parser" "$RUN_DIR/parser.pid" \
  env DATABASE_URL="$DATABASE_URL" REDIS_URL="$REDIS_URL" NATS_URL="$NATS_URL" \
      GRPC_ENDPOINT="$GRPC_ENDPOINT" GRPC_TOKEN="$GRPC_TOKEN" \
      CAPTURE_SAMPLES="${CAPTURE_SAMPLES:-0}" LISTEN_SECONDS="${LISTEN_SECONDS:-315360000}" \
  nohup "$PARSER_BIN" >> "$LOG_DIR/parser.log" 2>&1

echo
echo "日志目录: $LOG_DIR"
echo "查看状态: zsh scripts/local-status.sh"
echo "查看日志: zsh scripts/local-logs.sh"
echo "停止服务: zsh scripts/local-stop.sh"
