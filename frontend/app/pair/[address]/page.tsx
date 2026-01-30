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
    <div className="min-h-screen bg-[#0b0e11] text-white font-sans selection:bg-[#00cf9d]/30">
      <main className="grid grid-cols-1 lg:grid-cols-4 min-h-screen">
        {/* Left Col: Chart & Live Trades */}
        <div className="lg:col-span-3 flex flex-col border-r border-[#1a1e26]">
          {/* Chart Section */}
          <div className="h-[500px] border-b border-[#1a1e26]">
            <TradingChart
              data={candles}
              initialTimeframe={
                resolution as "1m" | "5m" | "15m" | "1h" | "4h" | "1d"
              }
              onTimeframeChange={(tf) => setResolution(tf)}
            />
          </div>

          {/* Live Trades Section */}
          <div className="flex-1 flex flex-col bg-[#0d1117]">
            {/* Section Header */}
            <div className="px-4 py-3 border-b border-[#1a1e26] bg-[#0d1117] flex items-center justify-between">
              <div className="flex items-center gap-2">
                <span className="h-2 w-2 rounded-full bg-[#00cf9d] animate-pulse shadow-[0_0_8px_#00cf9d]" />
                <h3 className="font-semibold text-white text-sm">
                  Live Trades
                </h3>
                <span className="text-xs text-gray-500">
                  ({trades.length.toLocaleString()})
                </span>
              </div>
            </div>

            {/* Table Header */}
            <div className="grid grid-cols-7 gap-0 px-4 py-2 bg-[#0b0e11] border-b border-[#1a1e26] text-[11px] font-medium uppercase tracking-wider text-gray-500">
              <div>Time</div>
              <div>Type</div>
              <div className="text-right">Price</div>
              <div className="text-right">Amount</div>
              <div className="text-right">SOL</div>
              <div>Maker</div>
              <div className="text-right">Tx</div>
            </div>

            {/* Virtualized Trade List */}
            <div
              ref={parentRef}
              className="flex-1 overflow-auto scrollbar-thin scrollbar-thumb-gray-700 scrollbar-track-transparent"
              style={{ height: TRADES_CONTAINER_HEIGHT }}
            >
              {trades.length === 0 ? (
                <div className="flex items-center justify-center h-full text-gray-600 text-sm">
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
                          "absolute left-0 top-0 w-full grid grid-cols-7 gap-0 px-4 items-center text-[13px] border-b border-[#1a1e26]/50 transition-colors duration-200",
                          "hover:bg-[#1a1e26]/50",
                          isNew && "animate-highlight",
                        )}
                        style={{
                          height: TRADE_ROW_HEIGHT,
                          transform: `translateY(${virtualRow.start}px)`,
                        }}
                      >
                        {/* Time */}
                        <div className="text-gray-400 font-mono text-xs">
                          {new Date(trade.time).toLocaleTimeString()}
                        </div>

                        {/* Type Badge */}
                        <div>
                          <span
                            className={clsx(
                              "inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-bold uppercase",
                              isBuy
                                ? "bg-[#00cf9d]/15 text-[#00cf9d]"
                                : "bg-[#ff4d4d]/15 text-[#ff4d4d]",
                            )}
                          >
                            {trade.type}
                          </span>
                        </div>

                        {/* Price */}
                        <div
                          className={clsx(
                            "text-right font-mono",
                            isBuy ? "text-[#00cf9d]" : "text-[#ff4d4d]",
                          )}
                        >
                          {formatPrice(trade.price)}
                        </div>

                        {/* Amount */}
                        <div className="text-right font-mono text-gray-300">
                          {formatAmount(trade.baseAmount)}
                        </div>

                        {/* SOL Value */}
                        <div
                          className={clsx(
                            "text-right font-mono font-medium",
                            isBuy ? "text-[#00cf9d]" : "text-[#ff4d4d]",
                          )}
                        >
                          {trade.quoteAmount.toFixed(3)}
                        </div>

                        {/* Maker Address */}
                        <div className="text-right font-mono text-gray-500 text-xs">
                          {formatAddress(trade.maker)}
                        </div>

                        {/* Tx Link */}
                        <div className="text-right">
                          <a
                            href={`https://solscan.io/tx/${trade.txHash}`}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="inline-flex items-center justify-center w-6 h-6 rounded hover:bg-[#1a1e26] text-gray-500 hover:text-[#00cf9d] transition-colors"
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
        <div className="lg:col-span-1 bg-[#0d1117]">
          <div className="p-4 border-b border-[#1a1e26]">
            <h3 className="text-sm font-semibold text-white">Token Info</h3>
          </div>
          <div className="p-4 space-y-3">
            {/* Placeholder Cards */}
            {["Pool Data", "Market Cap", "Liquidity", "Volume 24h"].map(
              (label) => (
                <div
                  key={label}
                  className="p-4 rounded-lg bg-[#1a1e26]/30 border border-[#1a1e26] hover:border-[#2a2e36] transition-colors"
                >
                  <div className="text-[11px] uppercase tracking-wider text-gray-500 mb-1">
                    {label}
                  </div>
                  <div className="text-lg font-semibold text-gray-400">--</div>
                </div>
              ),
            )}

            {/* Pool Address */}
            <div className="p-4 rounded-lg bg-[#1a1e26]/30 border border-[#1a1e26]">
              <div className="text-[11px] uppercase tracking-wider text-gray-500 mb-2">
                Pool Address
              </div>
              <a
                href={`https://solscan.io/account/${address}`}
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-2 text-sm font-mono text-[#00cf9d] hover:underline"
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
