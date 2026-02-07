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

export default function TokenDetailPage() {
  const parentRef = useRef<HTMLDivElement>(null);
  const tradesContainerRef = useRef<HTMLDivElement>(null);
  const params = useParams();
  const address = params?.address as string;

  // 新交易高亮追踪
  const [newTrades, setNewTrades] = useState<Set<string>>(new Set());
  const prevTradesRef = useRef<string[]>([]);

  // 交易列表容器高度（用于虚拟化）
  const [tradesListHeight, setTradesListHeight] = useState(400);

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

  // 计算交易列表容器高度
  useEffect(() => {
    const updateHeight = () => {
      if (tradesContainerRef.current) {
        const container = tradesContainerRef.current;
        // 获取 header 和 table header 的高度
        const sectionHeader = container.querySelector(
          ".flex.items-center.justify-between",
        );
        const tableHeader = container.querySelector(".table-header");
        const headerHeight =
          sectionHeader?.getBoundingClientRect().height || 48;
        const tableHeaderHeight =
          tableHeader?.getBoundingClientRect().height || 32;

        // 计算可用高度：视口高度 - 固定头部(56px) - Chart高度(在小屏幕上可能已滚动出视口) - section headers
        const viewportHeight = window.innerHeight;
        const chartHeight = 500; // Chart 固定高度
        const headerBarHeight = 56; // 顶部导航栏

        // 最小高度确保 trades 区域至少占满视口剩余部分
        const minTradesHeight =
          viewportHeight -
          headerBarHeight -
          chartHeight -
          headerHeight -
          tableHeaderHeight;
        // 但也不能太小
        const finalHeight = Math.max(200, minTradesHeight);

        setTradesListHeight(finalHeight);
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

  return (
    <main className="grid grid-cols-1 lg:grid-cols-4">
      {/* Left Col: Chart & Live Trades */}
      <div className="col-span-1 lg:col-span-3 flex flex-col lg:border-r border-(--border-primary) order-1">
        {/* Chart Section - 固定高度 */}
        <div className="h-[500px] border-b border-(--border-primary) shrink-0">
          <TradingChart
            data={candles}
            initialTimeframe={
              resolution as "1m" | "5m" | "15m" | "1h" | "4h" | "1d"
            }
            onTimeframeChange={(tf) => setResolution(tf)}
          />
        </div>

        {/* Live Trades Section - fixed height = viewport - header, so window scrolls exactly 500px (chart height) */}
        <div
          ref={tradesContainerRef}
          className="flex flex-col bg-(--bg-secondary) overflow-hidden"
          style={{ height: "calc(100vh - 56px)" }}
        >
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

          {/* Virtualized Trade List - uses CSS calc for responsive height */}
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

      {/* Right Col: Token Info - visible on all screen sizes */}
      <div className="col-span-1 bg-(--bg-secondary) order-2 lg:order-2 border-t lg:border-t-0 border-(--border-primary)">
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
  );
}
