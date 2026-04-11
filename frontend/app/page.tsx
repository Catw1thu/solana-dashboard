"use client";

import { PoolTable } from "../components/PoolTable";

export default function Home() {
  return (
    <main className="flex h-[calc(100vh-56px)] w-full min-h-0 flex-col overflow-hidden px-6 py-6">
      <div className="mb-6">
        <h2 className="text-2xl font-bold tracking-tight">市场总览</h2>
        <p className="mt-1 text-sm text-(--text-muted)">
          查看热门代币与最新代币。
        </p>
      </div>

      <div className="flex min-h-0 flex-1 flex-col">
        <PoolTable />
      </div>
    </main>
  );
}
