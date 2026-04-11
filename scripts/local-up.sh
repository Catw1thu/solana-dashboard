#!/bin/zsh
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
ENV_FILE="$ROOT_DIR/.env.local"
RUN_DIR="$ROOT_DIR/.run-local"
LOG_DIR="$ROOT_DIR/logs-local"
GO_DIR="$ROOT_DIR/solana-dashboard-go"
LAB_DIR="$ROOT_DIR/solana-dashboard-lab"
GO_BIN="$GO_DIR/bin/dashboard-api"
LAB_BIN="$LAB_DIR/target/release/solana-dashboard-lab"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "缺少 $ENV_FILE"
  echo "先执行：cp .env.local.example .env.local"
  exit 1
fi

source "$ENV_FILE"

: "${DATABASE_URL:?DATABASE_URL is required}"
: "${REDIS_URL:?REDIS_URL is required}"
: "${NATS_URL:?NATS_URL is required}"
: "${GRPC_ENDPOINT:?GRPC_ENDPOINT is required}"
: "${GRPC_TOKEN:?GRPC_TOKEN is required}"

mkdir -p "$RUN_DIR" "$LOG_DIR" "$GO_DIR/bin"

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

if [[ ! -x "$GO_BIN" ]]; then
  echo "构建 Go 二进制..."
  (
    cd "$GO_DIR"
    go build -o "$GO_BIN" ./cmd/api
  )
fi

if [[ ! -x "$LAB_BIN" ]]; then
  echo "构建 Rust release 二进制..."
  (
    cd "$LAB_DIR"
    cargo build --release
  )
fi

start_if_needed "dashboard-go" "$RUN_DIR/go.pid" \
  env DATABASE_URL="$DATABASE_URL" API_ADDR="${API_ADDR:-:8081}" NATS_URL="$NATS_URL" \
  nohup "$GO_BIN" >> "$LOG_DIR/go.log" 2>&1

start_if_needed "dashboard-lab" "$RUN_DIR/lab.pid" \
  env DATABASE_URL="$DATABASE_URL" REDIS_URL="$REDIS_URL" NATS_URL="$NATS_URL" \
      GRPC_ENDPOINT="$GRPC_ENDPOINT" GRPC_TOKEN="$GRPC_TOKEN" \
      CAPTURE_SAMPLES="${CAPTURE_SAMPLES:-0}" LISTEN_SECONDS="${LISTEN_SECONDS:-315360000}" \
  nohup "$LAB_BIN" >> "$LOG_DIR/lab.log" 2>&1

echo
echo "日志目录: $LOG_DIR"
echo "查看状态: zsh scripts/local-status.sh"
echo "查看日志: zsh scripts/local-logs.sh"
echo "停止服务: zsh scripts/local-stop.sh"
