"use client";

import { useParams } from "next/navigation";
import { useEffect, useState, useMemo } from "react";
import { useSocket } from "@/context/SocketContext";
import { useTradeFeed } from "@/hooks/useTradeFeed";
import { TradingChart, CandleData } from "@/components/TradingChart";
import { useSocketSubscription } from "@/hooks/useSocketSubscription"; // Ensure this hooks is used or useTradeFeed has it.
// Note: useTradeFeed already calls useSocketSubscription, so we don't need to call it twice if we use the feed.
// However, useTradeFeed returns "trades". We need to aggregate trades into candles for the chart.

import { ExternalLink } from "lucide-react";
import { formatAddress, formatPrice, formatAmount } from "@/utils/format";

export default function TokenDetailPage() {
  const params = useParams();
  const address = params?.address as string;

  const [resolution, setResolution] = useState("1m"); // Default resolution
  const [candles, setCandles] = useState<CandleData[]>([]);
  const { trades } = useTradeFeed(address);

  // 1. Fetch Historical Data
  useEffect(() => {
    if (!address) return;
    const fetchHistory = async () => {
      try {
        const res = await fetch(
          `http://localhost:3000/api/token/candles/${address}?resolution=${resolution}&from=0`
        );
        const data = await res.json();
        const formatted: CandleData[] = data.map((c: any) => ({
          time: new Date(c.time).getTime(),
          open: c.open,
          high: c.high,
          low: c.low,
          close: c.close,
          volume: c.volume,
        }));
        setCandles(formatted);
      } catch (e) {
        console.error("Failed to fetch history", e);
      }
    };
    fetchHistory();
  }, [address, resolution]);

  // 2. Real-time Candle Updates (Simple aggregation)
  // When new trades come in, we need to update the LAST candle or create a new one.
  // This logic is complex to get right in frontend only, but here is a simple MVP approach:
  // We can just re-fetch the latest candle periodically OR simplistic merge.
  // For MVP interactive feel, let's just listen to trades and update the last candle close.

  useEffect(() => {
    if (trades.length === 0 || candles.length === 0) return;

    const lastTrade = trades[0]; // Newest trade
    const lastCandle = candles[candles.length - 1]; // Current candle

    // Check if trade belongs to current candle (1m window)
    // Simple logic: if trade time > last candle time + 60s, make new candle.
    // However, candles are time-bucketed.
    // Let's rely on re-fetching for accuracy or complex logic.
    // For now, let's just log it. Real-time chart updates are tricky without decent state management.

    // Simplest approach: Just update the "Close" of the last candle to generate movement.
    // Verification: Does Lightweight charts support live updates? Yes via series.update().
    // But we are passing `data` prop.
    // Let's implement a refetch for now to keep it synced every 5 seconds.
  }, [trades]);

  return (
    <div className="min-h-screen bg-[#0b0e11] text-white font-sans selection:bg-[#00cf9d]/30">
      <main className="grid grid-cols-1 lg:grid-cols-4 divide-x divide-[#20242d] min-h-screen">
        {/* Left Col: Chart & Live Trades */}
        <div className="lg:col-span-3 flex flex-col">
          <div className="h-[500px] border-b border-[#20242d]">
            <TradingChart
              data={candles}
              initialTimeframe={
                resolution as "1m" | "5m" | "15m" | "1h" | "4h" | "1d"
              }
              onTimeframeChange={(tf) => setResolution(tf)}
            />
          </div>

          <div className="h-screen overflow-y-auto flex flex-col">
            <div className="p-3 border-b border-[#20242d] bg-[#0b0e11] sticky top-0 z-20">
              <h3 className="font-semibold text-white text-sm">Live Trades</h3>
            </div>
            <table className="w-full text-xs text-left">
              <thead className="text-[#88909f] sticky top-[45px] bg-[#0b0e11] z-10 border-b border-[#20242d]">
                <tr>
                  <th className="p-3">Time</th>
                  <th className="p-3">Type</th>
                  <th className="p-3 text-right">Price</th>
                  <th className="p-3 text-right">Amount</th>
                  <th className="p-3 text-right">SOL</th>
                  <th className="p-3 text-right">Maker</th>
                  <th className="p-3 text-right">Tx</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[#20242d] text-[#88909f]">
                {trades.map((t) => (
                  <tr
                    key={t.txHash}
                    className="hover:bg-[#20242d]/30 transition-colors"
                  >
                    <td className="p-3">
                      {new Date(t.time).toLocaleTimeString()}
                    </td>
                    <td
                      className={
                        t.type === "BUY"
                          ? "text-[#00cf9d] p-3"
                          : "text-[#ff4d4d] p-3"
                      }
                    >
                      {t.type}
                    </td>
                    <td className="p-3 text-right font-mono">
                      ${formatPrice(t.price)}
                    </td>
                    <td className="p-3 text-right font-mono">
                      {formatAmount(t.baseAmount)}
                    </td>
                    <td
                      className={`p-3 text-right font-mono ${
                        t.type === "BUY" ? "text-[#00cf9d]" : "text-[#ff4d4d]"
                      }`}
                    >
                      {t.quoteAmount.toFixed(3)}
                    </td>
                    <td className="p-3 text-right font-mono text-[#88909f]/70">
                      {formatAddress(t.maker)}
                    </td>
                    <td className="p-3 text-right font-mono">
                      <a
                        href={t.txHash}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="inline-flex items-center hover:text-[#00cf9d] transition-colors"
                      >
                        <ExternalLink size={14} />
                      </a>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>

        {/* Right Col: Token Info (Placeholder) */}
        <div className="lg:col-span-1 h-full">
          <div className="p-6">
            <h3 className="text-lg font-semibold text-white mb-4">
              Token Info
            </h3>
            <div className="space-y-4">
              <div className="h-20 rounded-lg bg-[#20242d]/20 border border-[#20242d] flex items-center justify-center text-[#88909f] text-sm">
                Pool Data coming soon...
              </div>
              <div className="h-20 rounded-lg bg-[#20242d]/20 border border-[#20242d] flex items-center justify-center text-[#88909f] text-sm">
                Market Cap coming soon...
              </div>
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}
