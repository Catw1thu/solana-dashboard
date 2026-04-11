"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import clsx from "clsx";

import { API } from "../config/api";
import { TokenListItem } from "../types";
import { formatPrice } from "../utils/format";

type TokenBoardView = "hot" | "new";
type TokenBoardWindow = "1m" | "5m" | "1h" | "4h" | "24h";

const TOKEN_LIMIT = 25;
const POLL_INTERVAL_MS = 1800;
const SOL_MINT = "So11111111111111111111111111111111111111112";

const VIEW_OPTIONS: Array<{ value: TokenBoardView; label: string }> = [
  { value: "hot", label: "热门" },
  { value: "new", label: "新币" },
];

const WINDOW_OPTIONS: Array<{ value: TokenBoardWindow; label: string }> = [
  { value: "1m", label: "1m" },
  { value: "5m", label: "5m" },
  { value: "1h", label: "1h" },
  { value: "4h", label: "4h" },
  { value: "24h", label: "24h" },
];

const VOLUME_THRESHOLDS = { warm: 18.29, hot: 55.85, blazing: 93.28 };
const TXN_THRESHOLDS = { warm: 28, hot: 89, blazing: 162 };
const LIQUIDITY_THRESHOLDS = { warm: 22.69, hot: 30, blazing: 34.37 };
const MARKET_CAP_THRESHOLDS = { warm: 28.29, hot: 31.07, blazing: 46.15 };

function formatAge(unixTs?: number | null): string {
  if (!unixTs) return "--";
  const ms = unixTs > 1e11 ? unixTs : unixTs * 1000;
  const diff = Math.max(0, Date.now() - ms);
  const seconds = Math.floor(diff / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h`;
  const days = Math.floor(hours / 24);
  return `${days}d`;
}

function formatDateTime(unixTs?: number | null): string {
  if (!unixTs) return "--";
  const ms = unixTs > 1e11 ? unixTs : unixTs * 1000;
  return new Date(ms).toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function formatCompactNumber(value?: number | null): string {
  if (value == null || !Number.isFinite(value)) return "--";
  const abs = Math.abs(value);
  if (abs >= 1_000_000_000) return `${(value / 1_000_000_000).toFixed(2)}B`;
  if (abs >= 1_000_000) return `${(value / 1_000_000).toFixed(2)}M`;
  if (abs >= 1_000) return `${(value / 1_000).toFixed(2)}K`;
  if (abs >= 1) return value.toFixed(2);
  return value.toFixed(4);
}

function formatQuoteValue(value?: number | null, quoteMint?: string): string {
  if (value == null || !Number.isFinite(value)) return "--";
  const suffix = !quoteMint || quoteMint === SOL_MINT ? " SOL" : "";
  return `${formatCompactNumber(value)}${suffix}`;
}

function formatSignedPercent(value?: number | null): string {
  if (value == null || !Number.isFinite(value)) return "--";
  const sign = value > 0 ? "+" : "";
  return `${sign}${value.toFixed(2)}%`;
}

function changeTone(value?: number | null): string {
  if (value == null || !Number.isFinite(value)) return "text-(--text-muted)";
  if (value > 0) return "text-(--accent-green)";
  if (value < 0) return "text-(--accent-red)";
  return "text-(--text-secondary)";
}

function heatLevel(
  value: number | null | undefined,
  thresholds: { warm: number; hot: number; blazing: number },
): 0 | 1 | 2 | 3 {
  if (value == null || !Number.isFinite(value) || value <= 0) return 0;
  if (value >= thresholds.blazing) return 3;
  if (value >= thresholds.hot) return 2;
  if (value >= thresholds.warm) return 1;
  return 0;
}

function volumeTone(level: 0 | 1 | 2 | 3): string {
  switch (level) {
    case 3:
      return "text-cyan-300";
    case 2:
      return "text-sky-400";
    case 1:
      return "text-teal-300";
    default:
      return "text-(--text-secondary)";
  }
}

function liquidityTone(level: 0 | 1 | 2 | 3): string {
  switch (level) {
    case 3:
      return "text-amber-300";
    case 2:
      return "text-orange-300";
    case 1:
      return "text-yellow-300";
    default:
      return "text-(--text-secondary)";
  }
}

function marketCapTone(level: 0 | 1 | 2 | 3): string {
  switch (level) {
    case 3:
      return "text-blue-300";
    case 2:
      return "text-sky-400";
    case 1:
      return "text-cyan-400";
    default:
      return "text-(--text-secondary)";
  }
}

function stageBadge(token: TokenListItem): { label: string; tone: string } {
  if (token.current_stage === "pool") {
    return {
      label: "PumpSwap",
      tone: "bg-emerald-500/12 text-emerald-300 ring-1 ring-emerald-500/20",
    };
  }

  return {
    label: "Pumpfun",
    tone: "bg-sky-500/12 text-sky-300 ring-1 ring-sky-500/20",
  };
}

function ageBadge(ageBase: number): { label: string; tone: string } {
  const age = formatAge(ageBase);
  const ms = ageBase > 1e11 ? ageBase : ageBase * 1000;
  const minutes = (Date.now() - ms) / 60000;
  if (minutes <= 10) {
    return { label: `🌱 ${age}`, tone: "text-emerald-300" };
  }
  if (minutes <= 60) {
    return { label: `⚡ ${age}`, tone: "text-amber-300" };
  }
  return { label: `🕒 ${age}`, tone: "text-(--text-muted)" };
}

export const PoolTable = () => {
  const router = useRouter();
  const [tokens, setTokens] = useState<TokenListItem[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [, setIsRefreshing] = useState(false);
  const [view, setView] = useState<TokenBoardView>("hot");
  const [selectedWindow, setSelectedWindow] = useState<TokenBoardWindow>("5m");
  const [failedImages, setFailedImages] = useState<Record<string, true>>({});
  const refreshTimerRef = useRef<number | null>(null);
  const inFlightRef = useRef(false);

  useEffect(() => {
    setFailedImages((current) => {
      const visibleMints = new Set(tokens.map((token) => token.mint));
      const next: Record<string, true> = {};
      let changed = false;

      for (const mint of Object.keys(current)) {
        if (visibleMints.has(mint)) {
          next[mint] = true;
        } else {
          changed = true;
        }
      }

      return changed ? next : current;
    });
  }, [tokens]);

  useEffect(() => {
    let cancelled = false;
    let controller: AbortController | null = null;

    const clearTimer = () => {
      if (refreshTimerRef.current != null) {
        globalThis.window.clearTimeout(refreshTimerRef.current);
        refreshTimerRef.current = null;
      }
    };

    const scheduleNext = () => {
      clearTimer();
      refreshTimerRef.current = globalThis.window.setTimeout(() => {
        void fetchTokens(true);
      }, POLL_INTERVAL_MS);
    };

    const fetchTokens = async (silent: boolean) => {
      if (inFlightRef.current) return;
      inFlightRef.current = true;
      controller = new AbortController();

      if (silent) {
        setIsRefreshing(true);
      } else {
        setIsLoading(true);
      }

      try {
        const res = await fetch(
          API.tokenList(TOKEN_LIMIT, view, selectedWindow),
          {
            cache: "no-store",
            signal: controller.signal,
          },
        );

        if (!res.ok) {
          throw new Error(`token list request failed: ${res.status}`);
        }

        const data = await res.json();
        if (!cancelled) {
          setTokens(Array.isArray(data.tokens) ? data.tokens : []);
        }
      } catch (error) {
        if (!cancelled) {
          console.error("Failed to fetch token board:", error);
        }
      } finally {
        inFlightRef.current = false;
        if (!cancelled) {
          setIsLoading(false);
          setIsRefreshing(false);
          scheduleNext();
        }
      }
    };

    void fetchTokens(false);

    return () => {
      cancelled = true;
      clearTimer();
      controller?.abort();
      inFlightRef.current = false;
    };
  }, [selectedWindow, view]);

  const emptyRows = useMemo(
    () => Math.max(0, TOKEN_LIMIT - tokens.length),
    [tokens.length],
  );

  return (
    <div className="flex h-full min-h-0 flex-1 flex-col overflow-hidden rounded-2xl border border-(--border-primary) bg-(--bg-secondary)">
      <div className="flex flex-col gap-4 border-b border-(--border-primary) px-6 py-5">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex flex-wrap items-center gap-3">
            <div className="flex rounded-xl border border-(--border-primary) bg-(--bg-tertiary) p-1">
              {VIEW_OPTIONS.map((option) => (
                <button
                  key={option.value}
                  type="button"
                  onClick={() => setView(option.value)}
                  className={clsx(
                    "rounded-lg px-4 py-2 text-sm font-medium transition-colors",
                    view === option.value
                      ? "bg-(--accent-green)/12 text-(--accent-green)"
                      : "text-(--text-muted) hover:text-(--text-primary)",
                  )}
                >
                  {option.label}
                </button>
              ))}
            </div>

            <div className="flex rounded-xl border border-(--border-primary) bg-(--bg-tertiary) p-1">
              {WINDOW_OPTIONS.map((option) => (
                <button
                  key={option.value}
                  type="button"
                  onClick={() => setSelectedWindow(option.value)}
                  className={clsx(
                    "rounded-lg px-3.5 py-2 text-sm font-medium transition-colors",
                    selectedWindow === option.value
                      ? "bg-(--accent-green)/12 text-(--accent-green)"
                      : "text-(--text-muted) hover:text-(--text-primary)",
                  )}
                >
                  {option.label}
                </button>
              ))}
            </div>
          </div>
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-auto overscroll-contain">
        <div className="min-h-full min-w-[1280px]">
          <div className="sticky top-0 z-10 grid grid-cols-[3.6fr_0.95fr_1fr_1.2fr_1.1fr_1.15fr_1.25fr_1.05fr] border-b border-(--border-primary) bg-(--bg-tertiary)/95 px-5 text-[12px] font-semibold uppercase tracking-[0.14em] text-(--text-muted) backdrop-blur">
            <div className="px-3 py-3.5">代币</div>
            <div className="px-3 py-3.5">价格</div>
            <div className="px-3 py-3.5">涨跌幅</div>
            <div className="px-3 py-3.5">流动性</div>
            <div className="px-3 py-3.5">市值</div>
            <div className="px-3 py-3.5">交易额</div>
            <div className="px-3 py-3.5">交易数</div>
            <div className="px-3 py-3.5">创建时间</div>
          </div>

          {isLoading && tokens.length === 0 ? (
            <div className="flex h-72 items-center justify-center text-sm text-(--text-muted)">
              加载代币中...
            </div>
          ) : tokens.length === 0 ? (
            <div className="flex h-72 items-center justify-center text-sm text-(--text-muted)">
              暂无代币数据
            </div>
          ) : (
            <div className="flex flex-col">
              {tokens.map((token) => {
                const displayName =
                  token.symbol || token.name || `${token.mint.slice(0, 4)}...`;
                const ageBase = token.active_since || token.accepted_at;
                const imageUrl = token.image_uri || token.uri;
                const showImage =
                  Boolean(imageUrl) && !failedImages[token.mint];
                const windowBuys = token.window_buys ?? 0;
                const windowSells = token.window_sells ?? 0;
                const stage = stageBadge(token);
                const age = ageBadge(ageBase);
                const volumeLevel = heatLevel(
                  token.window_volume,
                  VOLUME_THRESHOLDS,
                );
                const txnLevel = heatLevel(token.window_txns, TXN_THRESHOLDS);
                const liquidityLevel = heatLevel(
                  token.liquidity_quote,
                  LIQUIDITY_THRESHOLDS,
                );
                const marketCapLevel = heatLevel(
                  token.market_cap_quote,
                  MARKET_CAP_THRESHOLDS,
                );
                return (
                  <button
                    key={token.mint}
                    type="button"
                    onClick={() => router.push(`/token/${token.mint}`)}
                    className="grid min-h-[92px] grid-cols-[3.6fr_0.95fr_1fr_1.2fr_1.1fr_1.15fr_1.25fr_1.05fr] border-b border-(--border-primary)/60 px-5 text-left transition-colors hover:bg-(--bg-tertiary)"
                  >
                    <div className="flex items-center gap-4 px-3 py-4">
                      {showImage ? (
                        <img
                          src={imageUrl}
                          alt={token.symbol || "token"}
                          className="h-11 w-11 shrink-0 rounded-full object-cover ring-1 ring-(--border-primary)"
                          onError={() => {
                            setFailedImages((current) => ({
                              ...current,
                              [token.mint]: true,
                            }));
                          }}
                        />
                      ) : (
                        <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-(--bg-elevated) text-sm font-semibold text-(--text-muted) ring-1 ring-(--border-primary)">
                          {displayName[0]}
                        </div>
                      )}

                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <div className="truncate text-[18px] font-semibold text-(--text-primary)">
                            {displayName}
                          </div>
                        </div>
                        <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1">
                          <span
                            className={clsx(
                              "rounded-md px-2 py-0.5 text-[12px] font-semibold",
                              age.tone,
                            )}
                          >
                            {age.label}
                          </span>
                          <span className="truncate font-mono text-[12px] text-(--text-muted)">
                            {token.mint.slice(0, 4)}...{token.mint.slice(-4)}
                          </span>
                          <span
                            className={clsx(
                              "rounded-md px-2 py-0.5 text-[11px] font-medium",
                              stage.tone,
                            )}
                          >
                            {stage.label}
                          </span>
                        </div>
                      </div>
                    </div>

                    <div className="flex items-center px-3 py-4 font-mono text-[15px] text-(--text-primary)">
                      {token.latest_price != null
                        ? formatPrice(token.latest_price)
                        : "--"}
                    </div>

                    <div
                      className={clsx(
                        "flex items-center gap-1 px-3 py-4 font-mono text-[15px] font-semibold",
                        changeTone(token.price_change),
                      )}
                    >
                      {formatSignedPercent(token.price_change)}
                    </div>

                    <div
                      className={clsx(
                        "flex items-center gap-1 px-3 py-4 font-mono text-[15px]",
                        liquidityTone(liquidityLevel),
                      )}
                    >
                      {liquidityLevel > 0 ? (
                        <span>{liquidityLevel >= 2 ? "🔥" : "✨"}</span>
                      ) : null}
                      {formatQuoteValue(
                        token.liquidity_quote,
                        token.quote_mint,
                      )}
                    </div>

                    <div
                      className={clsx(
                        "flex items-center gap-1 px-3 py-4 font-mono text-[15px]",
                        marketCapTone(marketCapLevel),
                      )}
                    >
                      {formatQuoteValue(
                        token.market_cap_quote,
                        token.quote_mint,
                      )}
                    </div>

                    <div
                      className={clsx(
                        "flex items-center gap-1 px-3 py-4 font-mono text-[15px]",
                        volumeTone(volumeLevel),
                      )}
                    >
                      {formatQuoteValue(token.window_volume, token.quote_mint)}
                    </div>

                    <div className="flex flex-col justify-center px-3 py-4 font-mono">
                      <span
                        className={clsx(
                          "text-[17px] font-semibold",
                          txnLevel >= 2
                            ? "text-(--text-primary)"
                            : "text-(--text-secondary)",
                        )}
                      >
                        {token.window_txns.toLocaleString()}
                      </span>
                      <span className="mt-1 text-[13px]">
                        <span className="text-emerald-300">
                          {windowBuys.toLocaleString()}
                        </span>
                        <span className="px-1 text-(--text-muted)">/</span>
                        <span className="text-rose-300">
                          {windowSells.toLocaleString()}
                        </span>
                      </span>
                    </div>

                    <div className="flex items-center px-3 py-4 font-mono text-[14px] text-(--text-muted)">
                      {formatDateTime(token.accepted_at)}
                    </div>
                  </button>
                );
              })}

              {emptyRows > 0 &&
                Array.from({ length: emptyRows }).map((_, index) => (
                  <div
                    key={`empty-${index}`}
                    className="grid h-[92px] grid-cols-[3.6fr_0.95fr_1fr_1.1fr_1.1fr_1.15fr_1.25fr_1.05fr] border-b border-(--border-primary)/40 px-5 opacity-40"
                  >
                    <div className="mx-3 my-4 rounded-xl bg-(--bg-tertiary)" />
                    <div className="mx-3 my-5 rounded-lg bg-(--bg-tertiary)" />
                    <div className="mx-3 my-5 rounded-lg bg-(--bg-tertiary)" />
                    <div className="mx-3 my-5 rounded-lg bg-(--bg-tertiary)" />
                    <div className="mx-3 my-5 rounded-lg bg-(--bg-tertiary)" />
                    <div className="mx-3 my-5 rounded-lg bg-(--bg-tertiary)" />
                    <div className="mx-3 my-5 rounded-lg bg-(--bg-tertiary)" />
                    <div className="mx-3 my-5 rounded-lg bg-(--bg-tertiary)" />
                  </div>
                ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
