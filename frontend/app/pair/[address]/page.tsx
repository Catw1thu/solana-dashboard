"use client";

import { useParams } from "next/navigation";
import { useTradeFeed } from "@/hooks/useTradeFeed";
import { TradingChart } from "@/components/TradingChart";
import {
  ExternalLink,
  Copy,
  Check,
  TrendingUp,
  TrendingDown,
} from "lucide-react";
import { formatAddress, formatPrice, formatAmount } from "@/utils/format";
import { useVirtualizer } from "@tanstack/react-virtual";
import { useRef, useState, useEffect, useCallback } from "react";
import clsx from "clsx";
import { PoolDetail, PoolStats } from "@/types";
import { API } from "@/config/api";

const TRADE_ROW_HEIGHT = 36;

const STAT_WINDOWS = ["5m", "1h", "6h", "24h"] as const;
type StatWindow = (typeof STAT_WINDOWS)[number];

export default function TokenDetailPage() {
  const parentRef = useRef<HTMLDivElement>(null);
  const tradesContainerRef = useRef<HTMLDivElement>(null);
  const params = useParams();
  const address = params?.address as string;

  const [poolDetail, setPoolDetail] = useState<PoolDetail | null>(null);
  const [poolStats, setPoolStats] = useState<PoolStats | null>(null);
  const [statWindow, setStatWindow] = useState<StatWindow>("5m");
  const [copiedField, setCopiedField] = useState<string | null>(null);
  const [newTrades, setNewTrades] = useState<Set<string>>(new Set());
  const prevTradesRef = useRef<string[]>([]);
  const [tradesListHeight, setTradesListHeight] = useState(400);

  const { trades, candles, resolution, setResolution } = useTradeFeed(address);

  // Fetch pool detail & stats
  useEffect(() => {
    if (!address) return;
    const fetchData = async () => {
      try {
        const [poolRes, statsRes] = await Promise.all([
          fetch(API.pool(address)),
          fetch(API.stats(address)),
        ]);
        if (poolRes.ok) setPoolDetail(await poolRes.json());
        if (statsRes.ok) setPoolStats(await statsRes.json());
      } catch (e) {
        console.error("Failed to fetch pool data:", e);
      }
    };
    fetchData();

    // Refresh stats every 30s
    const interval = setInterval(async () => {
      try {
        const res = await fetch(API.stats(address));
        if (res.ok) setPoolStats(await res.json());
      } catch {
        /* ignore */
      }
    }, 30000);
    return () => clearInterval(interval);
  }, [address]);

  const copyToClipboard = useCallback((text: string, field: string) => {
    navigator.clipboard.writeText(text);
    setCopiedField(field);
    setTimeout(() => setCopiedField(null), 2000);
  }, []);

  // New trade highlight
  useEffect(() => {
    if (trades.length === 0) return;
    const currentHashes = trades.slice(0, 10).map((t) => t.txHash);
    const prevHashes = new Set(prevTradesRef.current);
    const newHashes = currentHashes.filter((h) => !prevHashes.has(h));
    if (newHashes.length > 0) {
      setNewTrades((prev) => new Set([...prev, ...newHashes]));
      setTimeout(() => {
        setNewTrades((prev) => {
          const next = new Set(prev);
          newHashes.forEach((h) => next.delete(h));
          return next;
        });
      }, 1000);
    }
    prevTradesRef.current = currentHashes;
  }, [trades]);

  const virtualizer = useVirtualizer({
    count: trades.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => TRADE_ROW_HEIGHT,
    overscan: 10,
  });

  useEffect(() => {
    const updateHeight = () => {
      if (tradesContainerRef.current) {
        const container = tradesContainerRef.current;
        const sectionHeader = container.querySelector(
          ".flex.items-center.justify-between",
        );
        const tableHeader = container.querySelector(".table-header");
        const headerHeight =
          sectionHeader?.getBoundingClientRect().height || 48;
        const tableHeaderHeight =
          tableHeader?.getBoundingClientRect().height || 32;
        const viewportHeight = window.innerHeight;
        const minTradesHeight =
          viewportHeight - 56 - 500 - headerHeight - tableHeaderHeight;
        setTradesListHeight(Math.max(200, minTradesHeight));
      }
    };
    updateHeight();
    window.addEventListener("resize", updateHeight);
    const timer = setTimeout(updateHeight, 100);
    return () => {
      window.removeEventListener("resize", updateHeight);
      clearTimeout(timer);
    };
  }, []);

  // --- Stat helpers keyed by window ---
  const getVolume = (w: StatWindow) => {
    if (!poolStats) return 0;
    return (
      {
        "5m": poolStats.volume5m,
        "1h": poolStats.volume1h,
        "6h": poolStats.volume6h,
        "24h": poolStats.volume24h,
      }[w] || 0
    );
  };

  const getTxns = (w: StatWindow) => {
    if (!poolStats) return 0;
    return (
      {
        "5m": poolStats.txns5m,
        "1h": poolStats.txns1h,
        "6h": poolStats.txns6h,
        "24h": poolStats.txns24h,
      }[w] || 0
    );
  };

  const getPriceChange = (w: StatWindow) => {
    if (!poolStats) return null;
    return {
      "5m": poolStats.priceChange5m,
      "1h": poolStats.priceChange1h,
      "6h": poolStats.priceChange6h,
      "24h": poolStats.priceChange24h,
    }[w];
  };

  const getBuys = (w: StatWindow) => {
    if (!poolStats) return 0;
    return (
      {
        "5m": poolStats.buys5m,
        "1h": poolStats.buys1h,
        "6h": poolStats.buys6h,
        "24h": poolStats.buys24h,
      }[w] || 0
    );
  };

  const getSells = (w: StatWindow) => {
    if (!poolStats) return 0;
    return (
      {
        "5m": poolStats.sells5m,
        "1h": poolStats.sells1h,
        "6h": poolStats.sells6h,
        "24h": poolStats.sells24h,
      }[w] || 0
    );
  };

  const formatAge = (dateStr: string) => {
    const diff = Date.now() - new Date(dateStr).getTime();
    const s = Math.floor(diff / 1000);
    if (s < 60) return `${s}s ago`;
    const m = Math.floor(s / 60);
    if (m < 60) return `${m}m ago`;
    const h = Math.floor(m / 60);
    if (h < 24) return `${h}h ago`;
    return `${Math.floor(h / 24)}d ago`;
  };

  const formatVolume = (vol: number) => {
    if (vol >= 1000) return `${(vol / 1000).toFixed(1)}K`;
    return vol.toFixed(2);
  };

  const priceChange = getPriceChange(statWindow);
  const isTrendUp = priceChange !== null && priceChange > 0;
  const isTrendDown = priceChange !== null && priceChange < 0;
  const isTrendFlat = priceChange !== null && priceChange === 0;
  const buys = getBuys(statWindow);
  const sells = getSells(statWindow);
  const totalBuySell = buys + sells;

  return (
    <main className="grid grid-cols-1 lg:grid-cols-4">
      {/* Left Col: Chart & Live Trades */}
      <div className="col-span-1 lg:col-span-3 flex flex-col lg:border-r border-(--border-primary) order-1">
        {/* Token Header Bar */}
        <div className="px-4 py-3 border-b border-(--border-primary) bg-(--bg-secondary) flex items-center gap-3">
          {poolDetail?.image ? (
            <img
              src={poolDetail.image}
              alt={poolDetail.symbol || "token"}
              className="w-8 h-8 rounded-full object-cover ring-2 ring-(--border-secondary) shrink-0"
              onError={(e) => {
                (e.target as HTMLImageElement).style.display = "none";
              }}
            />
          ) : (
            <div className="w-8 h-8 rounded-full bg-(--bg-tertiary) ring-2 ring-(--border-secondary) shrink-0 flex items-center justify-center text-sm text-(--text-muted) font-bold">
              {poolDetail?.symbol ? poolDetail.symbol[0] : "?"}
            </div>
          )}
          <div className="flex flex-col min-w-0">
            <div className="flex items-center gap-2">
              <span className="font-bold text-(--text-primary) text-lg leading-tight truncate">
                {poolDetail?.name || formatAddress(address)}
              </span>
              {poolDetail?.symbol && (
                <span className="badge bg-(--bg-tertiary) text-(--text-muted) text-xs">
                  {poolDetail.symbol}
                </span>
              )}
            </div>
            <div className="flex items-center gap-2 text-xs text-(--text-muted)">
              <span className="font-mono">{formatAddress(address)}</span>
              <button
                onClick={() => copyToClipboard(address, "pool")}
                className="hover:text-(--text-primary) transition-colors"
                title="Copy pool address"
              >
                {copiedField === "pool" ? (
                  <Check size={12} className="text-(--accent-green)" />
                ) : (
                  <Copy size={12} />
                )}
              </button>
              <a
                href={`https://solscan.io/account/${address}`}
                target="_blank"
                rel="noopener noreferrer"
                className="hover:text-(--accent-green) transition-colors"
              >
                <ExternalLink size={12} />
              </a>
            </div>
          </div>

          {/* Latest Price + Trend */}
          {poolStats?.price && (
            <div className="ml-auto flex items-center gap-3 shrink-0">
              <div className="text-right">
                <div className="font-mono font-bold text-(--text-primary) text-lg">
                  {formatPrice(poolStats.price)}
                </div>
                <div className="text-xs text-(--text-muted)">SOL</div>
              </div>
              {priceChange !== null && (
                <div
                  className={clsx(
                    "flex items-center gap-1 px-2 py-1 rounded-md text-sm font-semibold",
                    isTrendUp
                      ? "bg-(--accent-green)/10 text-(--accent-green)"
                      : isTrendDown
                        ? "bg-(--accent-red)/15 text-(--accent-red)"
                        : "bg-(--bg-tertiary) text-(--text-muted)",
                  )}
                >
                  {isTrendUp && <TrendingUp size={14} />}
                  {isTrendDown && <TrendingDown size={14} />}
                  {priceChange > 0 ? "+" : ""}
                  {priceChange.toFixed(2)}%
                </div>
              )}
            </div>
          )}
        </div>

        {/* Chart */}
        <div className="h-[500px] border-b border-(--border-primary) shrink-0">
          <TradingChart
            data={candles}
            initialTimeframe={
              resolution as "1m" | "5m" | "15m" | "1h" | "4h" | "1d"
            }
            onTimeframeChange={(tf) => setResolution(tf)}
          />
        </div>

        {/* Live Trades */}
        <div
          ref={tradesContainerRef}
          className="flex flex-col bg-(--bg-secondary) overflow-hidden"
          style={{ height: "calc(100vh - 56px)" }}
        >
          <div className="px-4 py-3 border-b border-(--border-primary) bg-(--bg-secondary) flex items-center justify-between">
            <div className="flex items-center gap-2">
              <span className="live-dot" />
              <h3 className="font-semibold text-(--text-primary) text-sm">
                Live Trades
              </h3>
              <span className="text-xs text-(--text-muted)">
                ({trades.length.toLocaleString()})
              </span>
            </div>
          </div>

          <div className="table-header grid grid-cols-7 gap-0 px-4 py-2">
            <div>Time</div>
            <div>Type</div>
            <div>Price</div>
            <div>Amount</div>
            <div>SOL</div>
            <div>Maker</div>
            <div className="text-right">Tx</div>
          </div>

          <div
            ref={parentRef}
            className="flex-1 overflow-auto scrollbar-thin"
            style={{ height: "calc(100vh - 56px - 48px - 32px)" }}
          >
            {trades.length === 0 ? (
              <div className="flex items-center justify-center h-full text-(--text-disabled) text-sm">
                Waiting for trades...
              </div>
            ) : (
              <div
                style={{
                  height: virtualizer.getTotalSize(),
                  width: "100%",
                  position: "relative",
                }}
              >
                {virtualizer.getVirtualItems().map((virtualRow) => {
                  const trade = trades[virtualRow.index];
                  const isBuy = trade.type === "BUY";
                  const isNew = newTrades.has(trade.txHash);

                  return (
                    <div
                      key={trade.txHash}
                      className={clsx(
                        "absolute left-0 top-0 w-full grid grid-cols-7 gap-0 px-4 items-center text-[13px] border-b border-(--border-primary)/50 transition-colors duration-200",
                        "hover:bg-(--bg-tertiary)",
                        isNew && "animate-highlight",
                      )}
                      style={{
                        height: TRADE_ROW_HEIGHT,
                        transform: `translateY(${virtualRow.start}px)`,
                      }}
                    >
                      <div className="text-(--text-muted) font-mono text-xs">
                        {new Date(trade.time).toLocaleTimeString()}
                      </div>
                      <div>
                        <span
                          className={clsx(
                            "badge",
                            isBuy ? "badge-buy" : "badge-sell",
                          )}
                        >
                          {trade.type}
                        </span>
                      </div>
                      <div
                        className={clsx(
                          "font-mono",
                          isBuy
                            ? "text-(--accent-green)"
                            : "text-(--accent-red)",
                        )}
                      >
                        {formatPrice(trade.price)}
                      </div>
                      <div className="font-mono text-(--text-secondary)">
                        {formatAmount(trade.baseAmount)}
                      </div>
                      <div
                        className={clsx(
                          "font-mono font-medium",
                          isBuy
                            ? "text-(--accent-green)"
                            : "text-(--accent-red)",
                        )}
                      >
                        {trade.quoteAmount.toFixed(3)}
                      </div>
                      <div className="font-mono text-(--text-muted) text-xs">
                        {formatAddress(trade.maker)}
                      </div>
                      <div className="text-right">
                        <a
                          href={`https://solscan.io/tx/${trade.txHash}`}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="inline-flex items-center justify-center w-6 h-6 rounded hover:bg-(--bg-tertiary) text-(--text-muted) hover:text-(--accent-green) transition-colors"
                        >
                          <ExternalLink size={12} />
                        </a>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Right Col: Token Info + Stats */}
      <div className="col-span-1 bg-(--bg-secondary) order-2 border-t lg:border-t-0 border-(--border-primary)">
        <div className="p-4 border-b border-(--border-primary)">
          <h3 className="text-sm font-semibold text-(--text-primary)">
            Token Info
          </h3>
        </div>
        <div className="p-4 space-y-3">
          {/* Time Window Tabs */}
          <div className="flex gap-1 p-1 rounded-lg bg-(--bg-tertiary)/50">
            {STAT_WINDOWS.map((w) => (
              <button
                key={w}
                onClick={() => setStatWindow(w)}
                className={clsx(
                  "flex-1 py-1.5 text-xs font-semibold rounded-md transition-all duration-200",
                  statWindow === w
                    ? "bg-(--accent-green)/10 text-(--accent-green)"
                    : "text-(--text-muted) hover:text-(--text-primary) hover:bg-(--bg-tertiary)",
                )}
              >
                {w}
              </button>
            ))}
          </div>

          {/* Price Change */}
          <div className="p-4 rounded-lg bg-(--bg-tertiary)/30 border border-(--border-primary)">
            <div className="text-[11px] uppercase tracking-wider text-(--text-muted) mb-1">
              Price Change ({statWindow})
            </div>
            <div className="flex items-center gap-2">
              <span
                className={clsx(
                  "text-xl font-bold font-mono",
                  priceChange === null || isTrendFlat
                    ? "text-(--text-muted)"
                    : isTrendUp
                      ? "text-(--accent-green)"
                      : "text-(--accent-red)",
                )}
              >
                {priceChange !== null
                  ? `${priceChange > 0 ? "+" : ""}${priceChange.toFixed(2)}%`
                  : "--"}
              </span>
              {priceChange !== null && isTrendUp && (
                <TrendingUp size={18} className="text-(--accent-green)" />
              )}
              {priceChange !== null && isTrendDown && (
                <TrendingDown size={18} className="text-(--accent-red)" />
              )}
            </div>
          </div>

          {/* Volume */}
          <div className="p-4 rounded-lg bg-(--bg-tertiary)/30 border border-(--border-primary)">
            <div className="text-[11px] uppercase tracking-wider text-(--text-muted) mb-1">
              Volume ({statWindow})
            </div>
            <div className="text-lg font-semibold font-mono text-(--text-primary)">
              {poolStats ? `${formatVolume(getVolume(statWindow))} SOL` : "--"}
            </div>
          </div>

          {/* Transactions */}
          <div className="p-4 rounded-lg bg-(--bg-tertiary)/30 border border-(--border-primary)">
            <div className="text-[11px] uppercase tracking-wider text-(--text-muted) mb-1">
              Transactions ({statWindow})
            </div>
            <div className="text-lg font-semibold font-mono text-(--text-primary)">
              {poolStats ? getTxns(statWindow).toLocaleString() : "--"}
            </div>
          </div>

          {/* Buy / Sell Ratio — per window */}
          <div className="p-4 rounded-lg bg-(--bg-tertiary)/30 border border-(--border-primary)">
            <div className="text-[11px] uppercase tracking-wider text-(--text-muted) mb-2">
              Buy / Sell ({statWindow})
            </div>
            {poolStats ? (
              <>
                <div className="flex justify-between text-sm font-mono mb-2">
                  <span className="text-(--accent-green)">{buys} buys</span>
                  <span className="text-(--accent-red)">{sells} sells</span>
                </div>
                {/* Ratio bar */}
                <div className="h-2 rounded-full overflow-hidden bg-(--bg-elevated) flex">
                  <div
                    className="bg-(--accent-green) transition-all duration-300"
                    style={{
                      width: `${totalBuySell > 0 ? (buys / totalBuySell) * 100 : 50}%`,
                    }}
                  />
                  <div className="flex-1 bg-(--accent-red)" />
                </div>
              </>
            ) : (
              <div className="text-lg font-semibold text-(--text-muted)">
                --
              </div>
            )}
          </div>

          {/* Age */}
          <div className="p-4 rounded-lg bg-(--bg-tertiary)/30 border border-(--border-primary)">
            <div className="text-[11px] uppercase tracking-wider text-(--text-muted) mb-1">
              Age
            </div>
            <div className="text-lg font-semibold text-(--text-primary)">
              {poolDetail?.createdAt ? formatAge(poolDetail.createdAt) : "--"}
            </div>
            <div className="text-xs text-(--text-muted) font-mono mt-0.5">
              {poolDetail?.createdAt
                ? new Date(poolDetail.createdAt).toLocaleString()
                : ""}
            </div>
          </div>

          {/* Addresses */}
          <InfoCard
            label="Pool Address"
            value={address}
            href={`https://solscan.io/account/${address}`}
            onCopy={() => copyToClipboard(address, "pool-info")}
            copied={copiedField === "pool-info"}
          />
          <InfoCard
            label="Token Mint"
            value={poolDetail?.baseMint || "--"}
            href={
              poolDetail?.baseMint
                ? `https://solscan.io/token/${poolDetail.baseMint}`
                : undefined
            }
            onCopy={
              poolDetail?.baseMint
                ? () => copyToClipboard(poolDetail.baseMint, "base-mint")
                : undefined
            }
            copied={copiedField === "base-mint"}
          />
        </div>
      </div>
    </main>
  );
}

function InfoCard({
  label,
  value,
  href,
  onCopy,
  copied,
}: {
  label: string;
  value: string;
  href?: string;
  onCopy?: () => void;
  copied?: boolean;
}) {
  const displayValue = value.length > 20 ? formatAddress(value) : value;
  return (
    <div className="p-4 rounded-lg bg-(--bg-tertiary)/30 border border-(--border-primary) hover:border-(--border-secondary) transition-colors">
      <div className="text-[11px] uppercase tracking-wider text-(--text-muted) mb-2">
        {label}
      </div>
      <div className="flex items-center gap-2">
        <span className="text-sm font-mono text-(--accent-green) truncate">
          {displayValue}
        </span>
        <div className="flex items-center gap-1 shrink-0">
          {onCopy && (
            <button
              onClick={onCopy}
              className="p-1 rounded hover:bg-(--bg-elevated) text-(--text-muted) hover:text-(--text-primary) transition-colors"
              title="Copy to clipboard"
            >
              {copied ? (
                <Check size={12} className="text-(--accent-green)" />
              ) : (
                <Copy size={12} />
              )}
            </button>
          )}
          {href && (
            <a
              href={href}
              target="_blank"
              rel="noopener noreferrer"
              className="p-1 rounded hover:bg-(--bg-elevated) text-(--text-muted) hover:text-(--accent-green) transition-colors"
              title="View on Solscan"
            >
              <ExternalLink size={12} />
            </a>
          )}
        </div>
      </div>
    </div>
  );
}
