"use client";

import { useParams } from "next/navigation";
import { useTradeFeed } from "@/hooks/useTradeFeed";
import { TradingChart } from "@/components/TradingChart";
import { ExternalLink } from "lucide-react";
import { formatAddress, formatPrice, formatAmount } from "@/utils/format";
import { useVirtualizer } from "@tanstack/react-virtual";
import { useRef, useState, useEffect } from "react";
import clsx from "clsx";

// 虚拟化配置
const TRADE_ROW_HEIGHT = 36;
const TRADES_CONTAINER_HEIGHT = 400;

export default function TokenDetailPage() {
  const parentRef = useRef<HTMLDivElement>(null);
  const params = useParams();
  const address = params?.address as string;

  // 新交易高亮追踪
  const [newTrades, setNewTrades] = useState<Set<string>>(new Set());
  const prevTradesRef = useRef<string[]>([]);

  // 数据源
  const { trades, candles, resolution, setResolution } = useTradeFeed(address);

  // 检测新交易并添加高亮
  useEffect(() => {
    if (trades.length === 0) return;

    const currentHashes = trades.slice(0, 10).map((t) => t.txHash);
    const prevHashes = new Set(prevTradesRef.current);

    const newHashes = currentHashes.filter((h) => !prevHashes.has(h));

    if (newHashes.length > 0) {
      setNewTrades((prev) => new Set([...prev, ...newHashes]));

      // 1秒后移除高亮
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

  // 虚拟化设置
  const virtualizer = useVirtualizer({
    count: trades.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => TRADE_ROW_HEIGHT,
    overscan: 10,
  });

  return (
    <div className="min-h-screen bg-(--bg-primary) text-(--text-primary) font-sans selection:bg-(--accent-green)/30">
      <main className="grid grid-cols-1 lg:grid-cols-4 min-h-screen">
        {/* Left Col: Chart & Live Trades */}
        <div className="lg:col-span-3 flex flex-col border-r border-(--border-primary)">
          {/* Chart Section */}
          <div className="h-[500px] border-b border-(--border-primary)">
            <TradingChart
              data={candles}
              initialTimeframe={
                resolution as "1m" | "5m" | "15m" | "1h" | "4h" | "1d"
              }
              onTimeframeChange={(tf) => setResolution(tf)}
            />
          </div>

          {/* Live Trades Section */}
          <div className="flex-1 flex flex-col bg-(--bg-secondary)">
            {/* Section Header */}
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

            {/* Table Header */}
            <div className="table-header grid grid-cols-7 gap-0 px-4 py-2">
              <div>Time</div>
              <div>Type</div>
              <div>Price</div>
              <div>Amount</div>
              <div>SOL</div>
              <div>Maker</div>
              <div className="text-right">Tx</div>
            </div>

            {/* Virtualized Trade List */}
            <div
              ref={parentRef}
              className="flex-1 overflow-auto scrollbar-thin"
              style={{ height: TRADES_CONTAINER_HEIGHT }}
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
                        {/* Time */}
                        <div className="text-(--text-muted) font-mono text-xs">
                          {new Date(trade.time).toLocaleTimeString()}
                        </div>

                        {/* Type Badge */}
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

                        {/* Price */}
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

                        {/* Amount */}
                        <div className="font-mono text-(--text-secondary)">
                          {formatAmount(trade.baseAmount)}
                        </div>

                        {/* SOL Value */}
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

                        {/* Maker Address */}
                        <div className="font-mono text-(--text-muted) text-xs">
                          {formatAddress(trade.maker)}
                        </div>

                        {/* Tx Link */}
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

        {/* Right Col: Token Info */}
        <div className="lg:col-span-1 bg-(--bg-secondary)">
          <div className="p-4 border-b border-(--border-primary)">
            <h3 className="text-sm font-semibold text-(--text-primary)">
              Token Info
            </h3>
          </div>
          <div className="p-4 space-y-3">
            {/* Placeholder Cards */}
            {["Pool Data", "Market Cap", "Liquidity", "Volume 24h"].map(
              (label) => (
                <div
                  key={label}
                  className="p-4 rounded-lg bg-(--bg-tertiary)/30 border border-(--border-primary) hover:border-(--border-secondary) transition-colors"
                >
                  <div className="text-[11px] uppercase tracking-wider text-(--text-muted) mb-1">
                    {label}
                  </div>
                  <div className="text-lg font-semibold text-(--text-muted)">
                    --
                  </div>
                </div>
              ),
            )}

            {/* Pool Address */}
            <div className="p-4 rounded-lg bg-(--bg-tertiary)/30 border border-(--border-primary)">
              <div className="text-[11px] uppercase tracking-wider text-(--text-muted) mb-2">
                Pool Address
              </div>
              <a
                href={`https://solscan.io/account/${address}`}
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-2 text-sm font-mono text-(--accent-green) hover:underline"
              >
                {formatAddress(address)}
                <ExternalLink size={12} />
              </a>
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}
