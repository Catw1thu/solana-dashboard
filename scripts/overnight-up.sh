#!/bin/zsh
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
ENV_FILE="$ROOT_DIR/.env.compose"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "缺少 $ENV_FILE"
  echo "先从 .env.compose.example 复制一份："
  echo "  cp .env.compose.example .env.compose"
  exit 1
fi

mkdir -p \
  "$ROOT_DIR/data/postgres" \
  "$ROOT_DIR/data/redis" \
  "$ROOT_DIR/data/nats"

cd "$ROOT_DIR"

docker compose --env-file "$ENV_FILE" up -d --build postgres redis nats migrate dashboard-go dashboard-lab

echo
echo "服务已启动。"
echo "查看状态:   ./scripts/overnight-status.sh"
echo "查看日志:   ./scripts/overnight-logs.sh"
echo "停止服务:   ./scripts/overnight-stop.sh"
