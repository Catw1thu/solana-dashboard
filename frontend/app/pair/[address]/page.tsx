"use client";

import { useParams } from "next/navigation";
import { useEffect, useState, useMemo } from "react";
import { useSocket } from "@/context/SocketContext";
import { useTradeFeed } from "@/hooks/useTradeFeed";
import { TradingChart, CandleData } from "@/components/TradingChart";
import { useSocketSubscription } from "@/hooks/useSocketSubscription"; // Ensure this hooks is used or useTradeFeed has it.
// Note: useTradeFeed already calls useSocketSubscription, so we don't need to call it twice if we use the feed.
// However, useTradeFeed returns "trades". We need to aggregate trades into candles for the chart.

export default function TokenDetailPage() {
  const params = useParams();
  const address = params?.address as string;

  const [candles, setCandles] = useState<CandleData[]>([]);
  const { trades } = useTradeFeed(address);

  // 1. Fetch Historical Data
  useEffect(() => {
    if (!address) return;
    const fetchHistory = async () => {
      try {
        const res = await fetch(
          `http://localhost:3000/api/token/candles/${address}?resolution=1m&from=0`
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
  }, [address]);

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
    <div className="min-h-screen bg-black text-white font-sans selection:bg-green-500/30">
      <header className="sticky top-0 z-50 border-b border-white/10 bg-black/50 backdrop-blur-xl">
        <div className="container mx-auto flex h-16 items-center justify-between px-4">
          <h1 className="text-xl font-bold tracking-tight">
            Token: {address.slice(0, 8)}...
          </h1>
        </div>
      </header>

      <main className="container mx-auto px-4 py-8 grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Left Col: Chart */}
        <div className="lg:col-span-2 flex flex-col gap-6">
          <TradingChart data={candles} />
        </div>

        {/* Right Col: Trade History */}
        <div className="lg:col-span-1">
          <div className="w-full rounded-xl border border-white/10 bg-white/5 backdrop-blur-md">
            <div className="p-4 border-b border-white/10">
              <h3 className="font-semibold text-white">Live Trades</h3>
            </div>
            <div className="max-h-[600px] overflow-y-auto">
              <table className="w-full text-xs text-left">
                <thead className="text-gray-500 sticky top-0 bg-black/90">
                  <tr>
                    <th className="p-3">Time</th>
                    <th className="p-3">Type</th>
                    <th className="p-3 text-right">Price</th>
                    <th className="p-3 text-right">SOL</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-white/5 text-gray-300">
                  {trades.map((t) => (
                    <tr
                      key={t.txHash}
                      className="hover:bg-white/5 transition-colors"
                    >
                      <td className="p-3 text-gray-500">
                        {new Date(t.time).toLocaleTimeString()}
                      </td>
                      <td
                        className={
                          t.type === "BUY"
                            ? "text-green-500 p-3"
                            : "text-red-500 p-3"
                        }
                      >
                        {t.type}
                      </td>
                      <td className="p-3 text-right font-mono text-blue-300">
                        ${t.price.toFixed(6)}
                      </td>
                      <td className="p-3 text-right font-mono">
                        {(t.volume / 1e9).toFixed(2)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}
