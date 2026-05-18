"use client";

import { useEffect, useMemo, useState } from "react";
import clsx from "clsx";
import {
  Activity,
  Cpu,
  Database,
  HardDrive,
  MemoryStick,
  Network,
  RefreshCcw,
  Server,
} from "lucide-react";

import { API } from "@/config/api";

const POLL_INTERVAL_MS = 2000;

interface DockerSnapshot {
  enabled: boolean;
  generated_at: number;
  container_prefix?: string;
  error?: string;
  totals: DockerTotals;
  containers: ContainerMetric[] | null;
  disk: DockerDiskUsage;
  runtime_dirs?: RuntimeDirUsage[] | null;
}

interface DockerTotals {
  containers: number;
  running: number;
  cpu_percent: number;
  memory_usage_bytes: number;
  memory_limit_bytes: number;
  network_rx_bytes: number;
  network_tx_bytes: number;
  block_read_bytes: number;
  block_write_bytes: number;
  writable_bytes: number;
  root_fs_bytes: number;
}

interface ContainerMetric {
  id: string;
  name: string;
  image: string;
  state: string;
  status: string;
  cpu_percent: number;
  memory_usage_bytes: number;
  memory_limit_bytes: number;
  memory_percent: number;
  network_rx_bytes: number;
  network_tx_bytes: number;
  block_read_bytes: number;
  block_write_bytes: number;
  writable_bytes: number;
  root_fs_bytes: number;
}

interface DockerDiskUsage {
  images_bytes: number;
  containers_bytes: number;
  volumes_bytes: number;
  build_cache_bytes: number;
  runtime_data_bytes?: number;
}

interface RuntimeDirUsage {
  name: string;
  path: string;
  bytes: number;
  error?: string;
}

const EMPTY_DISK: DockerDiskUsage = {
  images_bytes: 0,
  containers_bytes: 0,
  volumes_bytes: 0,
  build_cache_bytes: 0,
  runtime_data_bytes: 0,
};

function formatBytes(value?: number | null): string {
  if (value == null || !Number.isFinite(value) || value < 0) return "--";
  if (value === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const index = Math.min(
    units.length - 1,
    Math.floor(Math.log(value) / Math.log(1024)),
  );
  return `${(value / 1024 ** index).toFixed(index === 0 ? 0 : 2)} ${units[index]}`;
}

function formatPercent(value?: number | null): string {
  if (value == null || !Number.isFinite(value)) return "--";
  return `${value.toFixed(2)}%`;
}

function formatTime(unixTs?: number): string {
  if (!unixTs) return "--";
  return new Date(unixTs * 1000).toLocaleTimeString("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

function stateTone(state: string): string {
  switch (state) {
    case "running":
      return "bg-emerald-500/12 text-emerald-300 ring-1 ring-emerald-500/20";
    case "exited":
      return "bg-zinc-500/12 text-zinc-300 ring-1 ring-zinc-500/20";
    default:
      return "bg-amber-500/12 text-amber-300 ring-1 ring-amber-500/20";
  }
}

function ratio(value: number, total: number): number {
  if (!Number.isFinite(value) || !Number.isFinite(total) || total <= 0) {
    return 0;
  }
  return Math.max(0, Math.min(100, (value / total) * 100));
}

function StatTile({
  icon: Icon,
  label,
  value,
  detail,
  tone = "text-(--text-primary)",
}: {
  icon: typeof Activity;
  label: string;
  value: string;
  detail?: string;
  tone?: string;
}) {
  return (
    <div className="min-h-[108px] rounded-xl border border-(--border-primary) bg-(--bg-secondary) p-4">
      <div className="flex items-center justify-between">
        <span className="text-xs font-semibold uppercase tracking-[0.16em] text-(--text-muted)">
          {label}
        </span>
        <Icon className="h-4 w-4 text-(--text-muted)" />
      </div>
      <div className={clsx("mt-4 text-2xl font-semibold", tone)}>{value}</div>
      {detail && (
        <div className="mt-2 text-xs text-(--text-muted)">{detail}</div>
      )}
    </div>
  );
}

export default function OpsPage() {
  const [snapshot, setSnapshot] = useState<DockerSnapshot | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    let timer: number | null = null;
    let controller: AbortController | null = null;

    const fetchSnapshot = async () => {
      controller?.abort();
      controller = new AbortController();
      try {
        const res = await fetch(API.opsDocker(), {
          cache: "no-store",
          signal: controller.signal,
        });
        if (!res.ok) {
          throw new Error(`request failed: ${res.status}`);
        }
        const data = (await res.json()) as DockerSnapshot;
        if (!cancelled) {
          setSnapshot(data);
          setError(data.error ?? null);
          setIsLoading(false);
        }
      } catch (err) {
        if (!cancelled && (err as Error).name !== "AbortError") {
          setError((err as Error).message);
          setIsLoading(false);
        }
      } finally {
        if (!cancelled) {
          timer = window.setTimeout(fetchSnapshot, POLL_INTERVAL_MS);
        }
      }
    };

    void fetchSnapshot();

    return () => {
      cancelled = true;
      controller?.abort();
      if (timer != null) {
        window.clearTimeout(timer);
      }
    };
  }, []);

  const totals = snapshot?.totals;
  const disk = snapshot?.disk ?? EMPTY_DISK;
  const rawContainers = snapshot?.containers;
  const rawRuntimeDirs = snapshot?.runtime_dirs;
  const containers = useMemo(
    () => (Array.isArray(rawContainers) ? rawContainers : []),
    [rawContainers],
  );
  const runtimeDirs = useMemo(
    () => (Array.isArray(rawRuntimeDirs) ? rawRuntimeDirs : []),
    [rawRuntimeDirs],
  );
  const dockerDiskTotal = useMemo(() => {
    return (
      disk.images_bytes +
      disk.containers_bytes +
      disk.volumes_bytes +
      disk.build_cache_bytes
    );
  }, [disk]);
  const runtimeDataTotal = useMemo(() => {
    const fromDirs = runtimeDirs.reduce(
      (sum, dir) => sum + (dir.bytes ?? 0),
      0,
    );
    return fromDirs || disk.runtime_data_bytes || 0;
  }, [disk, runtimeDirs]);

  const memoryPercent = totals
    ? ratio(totals.memory_usage_bytes, totals.memory_limit_bytes)
    : 0;

  return (
    <main className="flex h-[calc(100vh-80px)] min-h-0 flex-col bg-(--bg-primary)">
      <section className="flex min-h-0 flex-1 flex-col gap-5 p-5">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <div className="flex items-center gap-3">
              <Server className="h-5 w-5 text-(--accent-green)" />
              <h1 className="text-xl font-semibold text-(--text-primary)">
                运行资源监控
              </h1>
            </div>
            <div className="mt-1 text-sm text-(--text-muted)">
              Docker 项目容器 CPU、内存、网络、块 IO 与磁盘占用
            </div>
          </div>
          <div className="flex items-center gap-3 rounded-xl border border-(--border-primary) bg-(--bg-secondary) px-3 py-2 text-sm text-(--text-muted)">
            <RefreshCcw
              className={clsx("h-4 w-4", isLoading && "animate-spin")}
            />
            <span>更新于 {formatTime(snapshot?.generated_at)}</span>
          </div>
        </div>

        {error && (
          <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 px-4 py-3 text-sm text-amber-200">
            {snapshot?.enabled === false
              ? "Docker 监控未启用。部署时设置 OPS_DOCKER_ENABLED=1 并挂载 /var/run/docker.sock 后可用。"
              : `监控数据暂不可用：${error}`}
          </div>
        )}

        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
          <StatTile
            icon={Cpu}
            label="CPU"
            value={formatPercent(totals?.cpu_percent)}
            detail="项目容器 CPU 占用合计"
            tone="text-sky-300"
          />
          <StatTile
            icon={MemoryStick}
            label="内存"
            value={formatBytes(totals?.memory_usage_bytes)}
            detail={`${formatPercent(memoryPercent)} / ${formatBytes(totals?.memory_limit_bytes)}`}
            tone="text-emerald-300"
          />
          <StatTile
            icon={Network}
            label="网络"
            value={formatBytes(
              (totals?.network_rx_bytes ?? 0) + (totals?.network_tx_bytes ?? 0),
            )}
            detail={`RX ${formatBytes(totals?.network_rx_bytes)} · TX ${formatBytes(totals?.network_tx_bytes)}`}
            tone="text-cyan-300"
          />
          <StatTile
            icon={HardDrive}
            label="运行数据"
            value={formatBytes(runtimeDataTotal)}
            detail={`Docker ${formatBytes(dockerDiskTotal)} · 可写层 ${formatBytes(totals?.writable_bytes)}`}
            tone="text-amber-300"
          />
        </div>

        <div className="grid min-h-0 flex-1 grid-cols-1 gap-5 xl:grid-cols-[1fr_360px]">
          <div className="min-h-0 overflow-hidden rounded-xl border border-(--border-primary) bg-(--bg-secondary)">
            <div className="flex items-center justify-between border-b border-(--border-primary) px-4 py-3">
              <div className="text-sm font-semibold text-(--text-primary)">
                容器明细
              </div>
              <div className="text-xs text-(--text-muted)">
                {totals?.running ?? 0}/{totals?.containers ?? 0} running
              </div>
            </div>
            <div className="min-h-0 overflow-auto">
              <table className="w-full min-w-[980px] text-left text-sm">
                <thead className="sticky top-0 z-10 bg-(--bg-primary) text-xs uppercase tracking-[0.14em] text-(--text-muted)">
                  <tr>
                    <th className="px-4 py-3 font-semibold">容器</th>
                    <th className="px-4 py-3 font-semibold">状态</th>
                    <th className="px-4 py-3 font-semibold">CPU</th>
                    <th className="px-4 py-3 font-semibold">内存</th>
                    <th className="px-4 py-3 font-semibold">网络</th>
                    <th className="px-4 py-3 font-semibold">块 IO</th>
                    <th className="px-4 py-3 font-semibold">可写层</th>
                  </tr>
                </thead>
                <tbody>
                  {containers.map((container) => (
                    <tr
                      key={container.id}
                      className="border-t border-(--border-primary) text-(--text-secondary)"
                    >
                      <td className="px-4 py-4">
                        <div className="font-semibold text-(--text-primary)">
                          {container.name.replace("solana-dashboard-", "")}
                        </div>
                        <div className="mt-1 max-w-[280px] truncate text-xs text-(--text-muted)">
                          {container.image}
                        </div>
                      </td>
                      <td className="px-4 py-4">
                        <span
                          className={clsx(
                            "inline-flex rounded-md px-2 py-1 text-xs font-semibold",
                            stateTone(container.state),
                          )}
                        >
                          {container.state || "--"}
                        </span>
                        <div className="mt-1 text-xs text-(--text-muted)">
                          {container.status || "--"}
                        </div>
                      </td>
                      <td className="px-4 py-4 font-mono text-sky-300">
                        {formatPercent(container.cpu_percent)}
                      </td>
                      <td className="px-4 py-4">
                        <div className="font-mono text-emerald-300">
                          {formatBytes(container.memory_usage_bytes)}
                        </div>
                        <div className="mt-1 h-1.5 w-28 overflow-hidden rounded-full bg-(--bg-tertiary)">
                          <div
                            className="h-full rounded-full bg-(--accent-green)"
                            style={{
                              width: `${ratio(
                                container.memory_usage_bytes,
                                container.memory_limit_bytes,
                              )}%`,
                            }}
                          />
                        </div>
                      </td>
                      <td className="px-4 py-4 font-mono text-xs">
                        <div>RX {formatBytes(container.network_rx_bytes)}</div>
                        <div className="mt-1 text-(--text-muted)">
                          TX {formatBytes(container.network_tx_bytes)}
                        </div>
                      </td>
                      <td className="px-4 py-4 font-mono text-xs">
                        <div>R {formatBytes(container.block_read_bytes)}</div>
                        <div className="mt-1 text-(--text-muted)">
                          W {formatBytes(container.block_write_bytes)}
                        </div>
                      </td>
                      <td className="px-4 py-4 font-mono text-amber-300">
                        {formatBytes(container.writable_bytes)}
                      </td>
                    </tr>
                  ))}
                  {containers.length === 0 && (
                    <tr>
                      <td
                        colSpan={7}
                        className="px-4 py-10 text-center text-sm text-(--text-muted)"
                      >
                        暂无匹配容器
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>

          <div className="grid content-start gap-4">
            <div className="rounded-xl border border-(--border-primary) bg-(--bg-secondary) p-4">
              <div className="flex items-center gap-2 text-sm font-semibold text-(--text-primary)">
                <HardDrive className="h-4 w-4 text-(--accent-yellow)" />
                运行数据目录
              </div>
              <div className="mt-4 space-y-3 text-sm">
                {runtimeDirs.map((dir) => (
                  <DiskRow
                    key={dir.path}
                    label={dir.name}
                    value={dir.bytes}
                    total={runtimeDataTotal}
                    detail={dir.error}
                  />
                ))}
                {runtimeDirs.length === 0 && (
                  <div className="text-sm text-(--text-muted)">
                    暂无目录数据。部署时挂载 ./data:/ops-data:ro 后可用。
                  </div>
                )}
              </div>
            </div>

            <div className="rounded-xl border border-(--border-primary) bg-(--bg-secondary) p-4">
              <div className="flex items-center gap-2 text-sm font-semibold text-(--text-primary)">
                <Database className="h-4 w-4 text-(--accent-yellow)" />
                Docker 磁盘拆分
              </div>
              <div className="mt-4 space-y-3 text-sm">
                <DiskRow
                  label="镜像"
                  value={disk.images_bytes}
                  total={dockerDiskTotal}
                />
                <DiskRow
                  label="容器"
                  value={disk.containers_bytes}
                  total={dockerDiskTotal}
                />
                <DiskRow
                  label="卷"
                  value={disk.volumes_bytes}
                  total={dockerDiskTotal}
                />
                <DiskRow
                  label="构建缓存"
                  value={disk.build_cache_bytes}
                  total={dockerDiskTotal}
                />
              </div>
            </div>

            <div className="rounded-xl border border-(--border-primary) bg-(--bg-secondary) p-4 text-sm text-(--text-muted)">
              <div className="font-semibold text-(--text-primary)">说明</div>
              <div className="mt-3 leading-6">
                当前页面读取 Docker Engine 的只读统计快照。CPU 为 Docker
                stats 瞬时占用，网络与块 IO 为容器累计值，磁盘占用来自 Docker
                system df 与容器可写层。运行数据目录来自 backend 只读挂载的
                /ops-data，并在后端缓存约 30 秒，避免频繁递归扫描。
              </div>
            </div>
          </div>
        </div>
      </section>
    </main>
  );
}

function DiskRow({
  label,
  value,
  total,
  detail,
}: {
  label: string;
  value?: number;
  total: number;
  detail?: string;
}) {
  const pct = ratio(value ?? 0, total);
  return (
    <div>
      <div className="mb-1 flex items-center justify-between gap-3">
        <span className="text-(--text-muted)">{label}</span>
        <span className="font-mono text-(--text-secondary)">
          {formatBytes(value)}
        </span>
      </div>
      <div className="h-1.5 overflow-hidden rounded-full bg-(--bg-tertiary)">
        <div
          className="h-full rounded-full bg-(--accent-yellow)"
          style={{ width: `${pct}%` }}
        />
      </div>
      {detail && <div className="mt-1 text-xs text-amber-300">{detail}</div>}
    </div>
  );
}
