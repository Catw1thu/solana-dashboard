"use client";

import { PoolTable } from "../components/PoolTable";
import { Activity } from "lucide-react";

export default function Home() {
  return (
    <div className="min-h-screen bg-black text-white font-sans selection:bg-green-500/30">
      {/* Header */}
      <header className="sticky top-0 z-50 border-b border-white/10 bg-black/50 backdrop-blur-xl">
        <div className="container mx-auto flex h-16 items-center justify-between px-4">
          <div className="flex items-center gap-2">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-green-500">
              <Activity className="h-5 w-5 text-black" />
            </div>
            <h1 className="text-xl font-bold tracking-tight">
              Solana Dashboard
            </h1>
          </div>
          <div className="flex items-center gap-4">
            <div className="text-xs font-medium text-green-500">
              ● System Operational
            </div>
          </div>
        </div>
      </header>

      <main className="container mx-auto px-4 py-8">
        <div className="mb-8">
          <h2 className="text-3xl font-bold tracking-tight">Market Overview</h2>
          <p className="mt-2 text-gray-400">
            Real-time feed of new liquidity pools on Solana.
          </p>
        </div>

        <PoolTable />
      </main>
    </div>
  );
}
