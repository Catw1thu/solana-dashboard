#!/bin/zsh
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
ENV_FILE="$ROOT_DIR/.env"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "缺少 $ENV_FILE"
  exit 1
fi

cd "$ROOT_DIR"

docker compose --env-file "$ENV_FILE" stop frontend parser backend postgres redis nats

echo "服务已停止，数据仍保留在 ./data 下。"
