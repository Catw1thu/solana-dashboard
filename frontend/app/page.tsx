"use client";

import { PoolTable } from "../components/PoolTable";

export default function Home() {
  return (
    <main className="container mx-auto px-4 py-6">
      {/* Page Title */}
      <div className="mb-6">
        <h2 className="text-2xl font-bold tracking-tight">Market Overview</h2>
        <p className="mt-1 text-sm text-(--text-muted)">
          Real-time feed of new liquidity pools on Solana.
        </p>
      </div>

      <PoolTable />
    </main>
  );
}
