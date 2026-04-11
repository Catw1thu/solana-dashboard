# Solana Dashboard

[中文](#中文说明) | [English](#english)

---

## 导航 / Navigation

- [中文说明](#中文说明)
- [English](#english)

## 中文说明

### 项目简介

`Solana Dashboard` 是一个面向 Solana meme coins 场景的实时监控系统，覆盖了从链上交易订阅、事件解析、数据库投影、实时指标维护，到前端榜单、详情页、K 线图和活动流展示的完整链路。

这个仓库是一个 monorepo，包含三部分：

- `parser/`：Rust 写的链上解析器，负责从 Yellowstone gRPC 订阅交易并产出统一事件。
- `backend/`：Go 写的 API / projector / realtime 服务，负责落库、投影、查询、WebSocket 推送和指标维护。
- `frontend/`：Next.js 前端，负责展示代币榜单、代币详情、实时 K 线和活动流。

### 适用场景

- 新币发现与监控
- Pumpfun / PumpSwap 生态实时观察
- 高频 token 价格、成交额、交易数、迁移事件可视化
- 全栈实时系统 / 数据工程 / 可观测性项目展示

### 技术栈

- Parser: Rust, Tokio, Yellowstone gRPC, NATS JetStream
- Backend: Go, pgx, TimescaleDB/PostgreSQL, WebSocket
- Frontend: Next.js, React 19, lightweight-charts
- Infra: Docker Compose, Redis, NATS, TimescaleDB

### 核心亮点

- 统一的 `service_event` protobuf 契约，贯穿 parser 与 backend
- `service_events -> projector -> read model` 的事件驱动读模型链路
- 首页榜单、详情静态摘要、实时 `token_stat` 统一到同一份指标真源
- dirty mint 增量刷新，只对有订阅的 mint 推送 WS 指标
- token-scoped WebSocket replay，支持 `since_log_id` 补齐断线窗口
- 多粒度 K 线、活动流分页、详情页增量更新联动
- monorepo 结构，便于统一维护协议、脚本和部署编排

---

### 系统架构

```mermaid
flowchart LR
  A["Yellowstone gRPC"] --> B["parser / Rust"]
  B --> C["NATS JetStream"]
  C --> D["backend ingest / Go"]
  D --> E["service_events"]
  E --> F["projector"]
  F --> G["read model"]
  F --> H["token_metrics_current"]
  G --> I["HTTP API"]
  H --> I
  H --> J["token_stat WS"]
  E --> K["token envelope WS / replay"]
  I --> L["frontend / Next.js"]
  J --> L
  K --> L
```

---

### 仓库结构

```text
solana-parser/
  frontend/               # Next.js 前端
  backend/                # Go API / projector / query / migrations
  parser/                 # Rust parser / tracker / emitter
  proto/                  # parser 与 backend 共享 protobuf 契约（单一真源）
  scripts/                # 跨服务脚本
  docker-compose.yml      # 本地开发编排（build 本地镜像）
  docker-compose.deploy.yml
  .env.example            # 根目录统一环境变量模板
  data/                   # 本地运行时数据目录（不属于源码）
  logs-local/             # 本地运行时日志目录（不属于源码）
```

### 重要约定

- 根目录 `proto/` 是协议单一真源，避免 parser / backend 各自维护副本。
- 根目录 `.env` 是唯一环境变量入口，不再在子项目各自维护独立 env。
- `data/` 和 `logs-local/` 只是本地运行时目录，不属于业务代码。

---

### 快速开始

如果你只想最快把整套系统在本地跑起来，推荐下面这组命令：

```bash
cp .env.example .env
docker compose up -d postgres redis nats
docker compose up migrate
```

然后分别在三个终端里启动：

```bash
cd backend && go run ./cmd/api
```

```bash
cd parser && cargo run
```

```bash
cd frontend && pnpm install && pnpm dev
```

说明：

- `data/` 用于本地 Postgres / Redis / NATS 持久化目录
- `logs-local/` 用于本地日志落盘或调试输出
- 这两个目录都属于本地运行时状态，不属于源码的一部分

---

### 核心数据流

#### 1. Parser（Rust）

- 通过 Yellowstone gRPC 订阅 Solana 交易流
- 解析 Pumpfun / PumpSwap 相关指令、事件和交易数据
- 封装成统一 `service_event` protobuf
- 发送到 NATS JetStream

对应目录：

- `parser/src/main.rs`
- `parser/src/service_event/`
- `parser/src/pumpfun/`
- `parser/src/pumpamm/`

#### 2. Ingest（Go）

- backend 从 JetStream 消费 protobuf 事件
- 事件先落到 `service_events`，作为原始事件真源
- 同时按 token topic 推送原始 envelope，供前端活动流 / 图表增量更新使用

对应目录：

- `backend/internal/jetstream/consumer.go`
- `backend/internal/ingest/service.go`
- `backend/internal/store/service_event.go`

#### 3. Projector（Go）

- projector 从 `service_events` 异步 replay
- 投影到 read model：
  - `tokens`
  - `token_metadata_current`
  - `token_markets`
  - `token_trade_events`
  - `token_activity_events`
  - `token_metrics_current`
- 使用 checkpoint 记录投影进度，支持冷启动恢复

对应目录：

- `backend/internal/projector/`
- `backend/internal/store/read_model.go`

#### 4. Query + WebSocket（Go）

- HTTP API 提供榜单、详情、K 线、活动流查询
- WebSocket 提供：
  - token 原始 envelope replay
  - `token_stat` 实时指标推送

对应目录：

- `backend/internal/query/`
- `backend/internal/httpapi/handler.go`
- `backend/internal/httpapi/ws.go`

#### 5. Frontend（Next.js）

- 首页榜单使用轮询模式
- 详情页先获取静态快照，再通过 WS 增量更新
- 活动流支持内部无限滚动
- K 线支持实时 patch 和分段历史加载

对应目录：

- `frontend/app/page.tsx`
- `frontend/app/token/[address]/page.tsx`
- `frontend/components/TradingChart.tsx`

---

### 一致性模型

这是整个系统里最重要的设计点之一。

#### 原始事件真源

- `service_events` 是原始事件真源。
- 所有 read model 和派生指标最终都以它为上游。

#### 读模型一致性

- `projector` 是异步的，所以 read model 是**最终一致**。
- 为了保证恢复能力，projector 使用 checkpoint 从上次 `log_id` 继续 replay。

#### 活动流一致性

- 详情页 activity API 返回 `snapshot_log_id`
- WebSocket 订阅支持 `since_log_id`
- 前端通过 snapshot + replay 合并，避免断线和首屏窗口造成的漏事件

#### 实时指标一致性

- 首页榜单、详情页静态摘要、详情页实时 `token_stat` 尽量使用统一指标语义
- 实时推送不是让前端自己算，而是后端计算后统一推送

---

### 指标系统设计

当前系统把代币指标分成两类：

#### 1. 当前状态型

- `latest_price`
- `latest_trade_at`
- `liquidity_quote`
- `market_cap_quote`

#### 2. 窗口统计型

- `1m / 5m / 1h / 4h / 24h`
- `volume`
- `txns`
- `buys`
- `sells`
- `buy_volume`
- `sell_volume`
- `anchor_price`

#### 真源

- `token_metrics_current` 是当前指标真源
- 首页榜单、详情页静态摘要、WS `token_stat` 都围绕这张 current metrics 表收口

#### 计算方式

- 短窗口（`1m / 5m`）使用 Go 内存秒桶做增量维护
- 长窗口（`1h / 4h / 24h`）从数据库聚合读取
- dirty mint 每秒批量刷新，只处理有变化的 mint
- 只对有订阅的 mint 推送实时 `token_stat`

#### 冷启动恢复

- projector 从 checkpoint 恢复 read model
- backend 启动时会对 current metrics 做 backfill
- 内存短窗口不是全量恢复，而是按需 seed

---

### K 线设计

#### 真源

- 成交真源是 `token_trade_events`

#### 聚合方式

- 支持多粒度 K 线查询
- 当前 K 线链路基于数据库聚合视图 / 连续聚合思路
- 不再对无成交区间强制补平 K（gapfill 已移除）

#### 前端加载方式

- 首屏加载一段最近历史
- 用户向左查看时分段加载更早历史
- 不再对整段历史做全量请求

#### 实时更新

- trade 类 envelope 直接 patch 当前 candle
- 活动和 K 线不再依赖每次都重新请求全量 candles

---

### 活动流设计

- 活动列表在详情页内部滚动，不影响整页高度
- 首屏加载最近一批活动
- 下滑时按 cursor 分页继续加载
- 前端收到 token envelope 后直接本地增量更新活动列表
- 用户查看历史时，新活动先缓存；回到顶部后再合并显示

---

### 本地开发

推荐两种模式。

#### 模式 A：推荐开发模式（infra 用 Docker，服务本地跑）

适合日常开发，启动快、调试方便。

1. 复制环境变量模板

```bash
cp .env.example .env
```

2. 启动基础设施

```bash
docker compose up -d postgres redis nats
docker compose up migrate
```

3. 启动 backend

```bash
cd backend
export DATABASE_URL='postgres://dashboard:dashboard@127.0.0.1:54329/solana_dashboard?sslmode=disable'
export API_ADDR=':8081'
export NATS_URL='nats://127.0.0.1:4222'
go run ./cmd/api
```

4. 启动 parser

```bash
cd parser
export DATABASE_URL='postgres://dashboard:dashboard@127.0.0.1:54329/solana_dashboard?sslmode=disable'
export REDIS_URL='redis://127.0.0.1:63799'
export NATS_URL='nats://127.0.0.1:4222'
export GRPC_ENDPOINT='https://solana-yellowstone-grpc.publicnode.com:443'
export GRPC_TOKEN='replace-with-your-yellowstone-token'
cargo run
```

5. 启动 frontend

```bash
cd frontend
pnpm install
pnpm dev
```

打开：

- Frontend: `http://127.0.0.1:3001`
- Backend: `http://127.0.0.1:8081`

#### 模式 B：全 Docker

```bash
cp .env.example .env
docker compose up --build
```

---

### Docker 与部署

#### 开发环境

- `docker-compose.yml` 使用本地 build
- 适合本地开发和联调

#### 部署环境

- `docker-compose.deploy.yml` 使用预构建镜像
- 适合服务器部署

#### 为什么数据库、Redis、NATS 用 Docker

- 依赖固定、易于一键启动
- 本地开发和服务器部署环境一致
- 降低安装和版本漂移成本

#### 为什么 frontend / backend / parser 在生产也建议容器化

- 部署方式统一
- 依赖和运行环境更可控
- 回滚、替换镜像和复现更简单

---

### 环境变量

根目录 `.env.example` 已按模块分组：

- Common
- Backend
- Parser
- Frontend
- Deployment images

关键变量包括：

- `DATABASE_URL`
- `REDIS_URL`
- `NATS_URL`
- `API_ADDR`
- `GRPC_ENDPOINT`
- `GRPC_TOKEN`
- `NEXT_PUBLIC_API_URL`
- `NEXT_PUBLIC_WS_URL`

说明：

- 根 `.env` 是 monorepo 唯一 env 入口
- 不再在 `frontend/`、`backend/`、`parser/` 里各自维护独立 env

---

### 数据库迁移

当前数据库迁移采用：

- 原生 SQL migration 文件
- `migrate` 容器统一执行

这套方式对当前项目是合适的，因为：

- 数据库 heavily 依赖 PostgreSQL / TimescaleDB 特性
- 有 hypertable、连续聚合、原生 SQL 逻辑
- 不适合强行套 Prisma 这类 ORM 驱动迁移工具

#### 本地执行迁移

```bash
docker compose up migrate
```

#### 部署时执行迁移

部署 compose 会先跑 `migrate`，成功后再启动 backend / parser。

---

### API 与 WebSocket

#### 主要 HTTP API

- `GET /healthz`
- `GET /metrics`
- `GET /tokens`
- `GET /search/tokens`
- `GET /tokens/{mint}`
- `GET /tokens/{mint}/events`
- `GET /tokens/{mint}/candles`
- `GET /tokens/{mint}/activity`
- `GET /tokens/{mint}/trades`

#### WebSocket

- `subscribe`
- `unsubscribe`
- `since_log_id`
- token envelope replay
- `token_stat` 实时指标推送

设计目标：

- 初始快照与增量同步对齐
- 连接断开后可补 replay
- 避免静默丢事件

---

### 可观测性

backend 提供 `/metrics` 端点，用于输出轻量级运行指标。

重点指标包括：

- ingest 延迟
- projector lag
- WS replay 次数
- WS overflow 次数
- dirty mint / refresh target 数量
- token metrics backfill 统计
- API 处理延迟

这套指标主要用于：

- 判断瓶颈在 ingest、projector、DB 还是 WS
- 观察冷启动恢复和 replay 情况
- 为后续性能优化提供依据

---

### 当前做过的工程优化

- token-scoped WebSocket replay
- activity snapshot + replay 一致性补齐
- `token_metrics_current` 指标真源化
- dirty mint 增量刷新
- 只对订阅中的 mint 推送 WS 指标
- projector 事务化与批量写入
- 启动时 current metrics backfill
- 详情页活动流无限滚动
- K 线分段加载，避免全量历史请求
- 详情页相对时间刷新从 page 级 state 下沉，避免整页每秒 rerender

---

### 已知取舍 / 当前未完成项

- 暂未做死币归档、冷热分层和数据库压缩策略
- 暂未做多实例 WS fanout
- 暂未引入 Redis 作为核心指标真源
- 首页榜单仍采用轮询而非 WS
- 生产级监控和告警系统尚未完整接入

这些都是有意的工程取舍，当前目标是优先保证：

- 实时链路正确
- 指标语义统一
- 项目结构清晰
- 本地和部署流程稳定

---


## English

### Overview

`Solana Dashboard` is a real-time full-stack monitoring system for Solana meme coins. It covers the complete pipeline from on-chain transaction subscription, event parsing, database projection, and real-time metric maintenance to frontend dashboards, token detail pages, candlestick charts, and activity feeds.

This repository is a monorepo with three major parts:

- `parser/`: a Rust parser that subscribes to Yellowstone gRPC and emits normalized service events
- `backend/`: a Go service that handles ingestion, projection, querying, metrics, and WebSocket delivery
- `frontend/`: a Next.js application that renders token boards, token detail pages, live charts, and activity streams

### Quick Start

If you only want to get the system running locally as quickly as possible:

```bash
cp .env.example .env
docker compose up -d postgres redis nats
docker compose up migrate
```

Then start the three app processes in separate terminals:

```bash
cd backend && go run ./cmd/api
```

```bash
cd parser && cargo run
```

```bash
cd frontend && pnpm install && pnpm dev
```

Notes:

- `data/` stores local runtime state for Postgres / Redis / NATS
- `logs-local/` stores local log output
- both directories are runtime-only and are not treated as source code

### Tech Stack

- Parser: Rust, Tokio, Yellowstone gRPC, NATS JetStream
- Backend: Go, pgx, TimescaleDB/PostgreSQL, WebSocket
- Frontend: Next.js, React 19, lightweight-charts
- Infra: Docker Compose, Redis, NATS, TimescaleDB

### Repository Layout

```text
solana-parser/
  frontend/               # Next.js frontend
  backend/                # Go API / projector / queries / migrations
  parser/                 # Rust parser / tracker / emitter
  proto/                  # shared protobuf contract (single source of truth)
  scripts/                # cross-service scripts
  docker-compose.yml
  docker-compose.deploy.yml
  .env.example
  data/                   # local runtime state only
  logs-local/             # local runtime logs only
```

### Architecture

```mermaid
flowchart LR
  A["Yellowstone gRPC"] --> B["parser / Rust"]
  B --> C["NATS JetStream"]
  C --> D["backend ingest / Go"]
  D --> E["service_events"]
  E --> F["projector"]
  F --> G["read model"]
  F --> H["token_metrics_current"]
  G --> I["HTTP API"]
  H --> I
  H --> J["token_stat WS"]
  E --> K["token envelope WS / replay"]
  I --> L["frontend / Next.js"]
  J --> L
  K --> L
```

### Core Data Flow

1. The Rust parser subscribes to Yellowstone gRPC and parses Pumpfun / PumpSwap related activity.
2. Parsed events are encoded with a shared protobuf schema and sent into NATS JetStream.
3. The Go backend consumes those events, stores them in `service_events`, and republishes token-scoped realtime envelopes.
4. The projector asynchronously builds the read model and updates `token_metrics_current`.
5. The frontend fetches snapshots via HTTP and keeps charts, token stats, and activity feeds updated via WebSocket.

### Consistency Model

- `service_events` is the raw source of truth.
- Read models are eventually consistent and recovered from projector checkpoints.
- Activity feed uses `snapshot_log_id` + `since_log_id` replay to bridge initial snapshot gaps.
- Realtime stats are produced by the backend and pushed as `token_stat`, instead of being recomputed in the client.

### Metrics System

The project distinguishes two kinds of token metrics:

- Current-state metrics:
  - `latest_price`
  - `latest_trade_at`
  - `liquidity_quote`
  - `market_cap_quote`

- Windowed metrics:
  - `1m / 5m / 1h / 4h / 24h`
  - volume
  - txns
  - buys / sells
  - buy_volume / sell_volume
  - anchor_price

`token_metrics_current` acts as the main current metrics table. Short windows are maintained in memory with per-second buckets, long windows are loaded from the database, and dirty mints are refreshed in batches once per second.

### Candlestick Design

- Raw trades come from `token_trade_events`
- Charts support multiple resolutions
- History is loaded incrementally rather than fetched in a single full-range request
- Realtime trade envelopes patch the current visible candle directly

### Activity Feed Design

- The activity list scrolls inside the detail page instead of expanding page height
- Initial page data is fetched through HTTP
- Older activity pages are loaded via cursor pagination
- Realtime envelopes are applied locally on the client
- When a user is reading older history, new activity is buffered and merged when they return to the top

### Local Development

#### Recommended Mode: Docker for infra, native processes for app services

```bash
cp .env.example .env
docker compose up -d postgres redis nats
docker compose up migrate
```

Backend:

```bash
cd backend
export DATABASE_URL='postgres://dashboard:dashboard@127.0.0.1:54329/solana_dashboard?sslmode=disable'
export API_ADDR=':8081'
export NATS_URL='nats://127.0.0.1:4222'
go run ./cmd/api
```

Parser:

```bash
cd parser
export DATABASE_URL='postgres://dashboard:dashboard@127.0.0.1:54329/solana_dashboard?sslmode=disable'
export REDIS_URL='redis://127.0.0.1:63799'
export NATS_URL='nats://127.0.0.1:4222'
export GRPC_ENDPOINT='https://solana-yellowstone-grpc.publicnode.com:443'
export GRPC_TOKEN='replace-with-your-yellowstone-token'
cargo run
```

Frontend:

```bash
cd frontend
pnpm install
pnpm dev
```

#### Full Docker Mode

```bash
cp .env.example .env
docker compose up --build
```

### Environment Variables

The root `.env.example` is the single environment entry point for the monorepo. Variables are grouped by:

- Common
- Backend
- Parser
- Frontend
- Deployment images

### Database Migrations

Database migrations are managed with raw SQL files plus a dedicated `migrate` container. This approach is intentionally chosen because the project relies heavily on PostgreSQL / TimescaleDB features such as hypertables and continuous aggregates.

### API / WebSocket

Main HTTP routes:

- `GET /healthz`
- `GET /metrics`
- `GET /tokens`
- `GET /search/tokens`
- `GET /tokens/{mint}`
- `GET /tokens/{mint}/events`
- `GET /tokens/{mint}/candles`
- `GET /tokens/{mint}/activity`
- `GET /tokens/{mint}/trades`

WebSocket features:

- `subscribe`
- `unsubscribe`
- `since_log_id`
- token envelope replay
- realtime `token_stat`

### Observability

The backend exposes a lightweight `/metrics` endpoint and tracks:

- ingest latency
- projector lag
- replay count / overflow count
- dirty mint refresh count
- current metrics backfill
- API latency

### Engineering Tradeoffs

- No dead-token archival / database compression yet
- No multi-instance WebSocket fanout yet
- Redis is not used as the core realtime source of truth
- Token board still uses polling instead of full WS sync
- Full production-grade monitoring and alerting is not finished yet

These are deliberate tradeoffs for the current project phase: correctness, clarity, and stable realtime behavior are prioritized over full production hardening.

