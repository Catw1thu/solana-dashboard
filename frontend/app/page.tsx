"use client";

import { PoolTable } from "../components/PoolTable";
import { Activity } from "lucide-react";

export default function Home() {
  return (
    <div className="min-h-screen bg-[var(--bg-primary)] text-[var(--text-primary)] font-sans selection:bg-[var(--accent-green)]/30">
      {/* Header */}
      <header className="sticky top-0 z-50 border-b border-[var(--border-primary)] bg-[var(--bg-primary)]/80 backdrop-blur-xl">
        <div className="container mx-auto flex h-14 items-center justify-between px-4">
          <div className="flex items-center gap-3">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-[var(--accent-green)]">
              <Activity className="h-4 w-4 text-black" />
            </div>
            <h1 className="text-lg font-bold tracking-tight">
              Solana Dashboard
            </h1>
          </div>
          <div className="flex items-center gap-4">
            <div className="flex items-center gap-2 text-xs font-medium text-[var(--accent-green)]">
              <span className="live-dot" />
              System Operational
            </div>
          </div>
        </div>
      </header>

      <main className="container mx-auto px-4 py-6">
        {/* Page Title */}
        <div className="mb-6">
          <h2 className="text-2xl font-bold tracking-tight">Market Overview</h2>
          <p className="mt-1 text-sm text-[var(--text-muted)]">
            Real-time feed of new liquidity pools on Solana.
          </p>
        </div>

        <PoolTable />
      </main>
    </div>
  );
}
