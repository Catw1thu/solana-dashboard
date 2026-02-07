"use client";

import { PoolTable } from "../components/PoolTable";

export default function Home() {
  return (
    <main className="w-full px-6 py-6 h-[calc(100vh-56px)] flex flex-col overflow-hidden">
      {/* Page Title */}
      <div className="mb-6">
        <h2 className="text-2xl font-bold tracking-tight">Market Overview</h2>
        <p className="mt-1 text-sm text-(--text-muted)">
          Real-time feed of new liquidity pools on Solana.
        </p>
      </div>

      <div className="flex-1 flex flex-col">
        <PoolTable />
      </div>
    </main>
  );
}
