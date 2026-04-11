#!/bin/zsh
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
ENV_FILE="$ROOT_DIR/.env"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "缺少 $ENV_FILE"
  exit 1
fi

cd "$ROOT_DIR"

docker compose --env-file "$ENV_FILE" ps
echo
echo "数据目录占用:"
du -sh "$ROOT_DIR/data/postgres" "$ROOT_DIR/data/redis" "$ROOT_DIR/data/nats" 2>/dev/null || true
