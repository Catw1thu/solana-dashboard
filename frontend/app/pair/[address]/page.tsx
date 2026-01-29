"use client";

import { useParams } from "next/navigation";
import { useTradeFeed } from "@/hooks/useTradeFeed";
import { TradingChart } from "@/components/TradingChart";

import { ExternalLink } from "lucide-react";
import { formatAddress, formatPrice, formatAmount } from "@/utils/format";

export default function TokenDetailPage() {
  const params = useParams();
  const address = params?.address as string;

  // Unified data source for trades and candles
  const { trades, candles, resolution, setResolution } = useTradeFeed(address);

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
