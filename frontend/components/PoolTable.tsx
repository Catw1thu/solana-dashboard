"use client";

import { useState, useEffect, useRef } from "react";
import { useSocket } from "../context/SocketContext";
import { useSocketSubscription } from "../hooks/useSocketSubscription";
import { PoolData } from "../types";
import { Play } from "lucide-react";
import clsx from "clsx";
import { useRouter } from "next/navigation";
import { useVirtualizer } from "@tanstack/react-virtual";

// Row height in pixels - must be consistent for virtualization
const ROW_HEIGHT = 52;
// Maximum number of pools to keep in memory
const MAX_POOLS = 500;
// Container height for the virtual list
const CONTAINER_HEIGHT = 400;

export const PoolTable = () => {
  const { socket } = useSocket();
  const router = useRouter();
  const [pools, setPools] = useState<PoolData[]>([]);
  // Track newly added pools for highlight animation
  const [newPoolAddresses, setNewPoolAddresses] = useState<Set<string>>(
    new Set(),
  );

  // Reference to the scrollable container
  const parentRef = useRef<HTMLDivElement>(null);

  // 1. Subscribe to Global Room
  useSocketSubscription("room:global");

  // 2. Listen for new pools
  useEffect(() => {
    if (!socket) return;

    const handleNewPool = (pool: PoolData) => {
      console.log("New Pool:", pool);

      // Add to new pools set for highlight
      setNewPoolAddresses((prev) => new Set(prev).add(pool.address));

      // Remove highlight after animation completes
      setTimeout(() => {
        setNewPoolAddresses((prev) => {
          const next = new Set(prev);
          next.delete(pool.address);
          return next;
        });
      }, 1000);

      setPools((prev) => [pool, ...prev].slice(0, MAX_POOLS));
    };

    socket.on("pool:new", handleNewPool);

    return () => {
      socket.off("pool:new", handleNewPool);
    };
  }, [socket]);

  // 3. Fetch initial data from API
  useEffect(() => {
    const fetchPools = async () => {
      try {
        const res = await fetch(
          "http://localhost:3000/api/token/pools?limit=50",
        );
        const data = await res.json();

        const mappedPools: PoolData[] = data.map((p: any) => ({
          address: p.address,
          mint: p.baseMint,
          solAmount: "0",
          tokenAmount: "0",
          timestamp: new Date(p.createdAt).getTime(),
        }));

        setPools(mappedPools);
      } catch (err) {
        console.error("Failed to fetch initial pools:", err);
      }
    };

    fetchPools();
  }, []);

  // 4. Setup virtualizer
  const virtualizer = useVirtualizer({
    count: pools.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 5, // Render 5 extra items above/below viewport
  });

  const virtualItems = virtualizer.getVirtualItems();

  return (
    <div className="w-full overflow-hidden rounded-xl border border-white/10 bg-white/5 backdrop-blur-md">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-white/10 px-6 py-4">
        <h2 className="flex items-center gap-2 text-lg font-semibold text-white">
          <Play className="h-5 w-5 fill-green-500 text-green-500" />
          Live New Pools
          <span className="ml-2 text-sm font-normal text-gray-500">
            ({pools.length})
          </span>
        </h2>
        <span className="flex h-2 w-2 animate-pulse rounded-full bg-green-500 shadow-[0_0_10px_#22c55e]"></span>
      </div>

      {/* Table Header (Fixed) */}
      <div className="bg-white/5 text-xs uppercase text-gray-200">
        <div className="flex">
          <div className="w-1/4 px-6 py-3 font-medium">Pool Address</div>
          <div className="w-1/4 px-6 py-3 font-medium">Token Mint</div>
          <div className="w-1/4 px-6 py-3 font-medium">Initial SOL</div>
          <div className="w-1/4 px-6 py-3 font-medium text-right">Time</div>
        </div>
      </div>

      {/* Virtualized Table Body */}
      <div
        ref={parentRef}
        className="overflow-auto"
        style={{ height: CONTAINER_HEIGHT }}
      >
        {pools.length === 0 ? (
          <div className="px-6 py-8 text-center text-gray-600">
            Waiting for new pools...
          </div>
        ) : (
          <div
            style={{
              height: `${virtualizer.getTotalSize()}px`,
              width: "100%",
              position: "relative",
            }}
          >
            {virtualItems.map((virtualRow) => {
              const pool = pools[virtualRow.index];
              const isNew = newPoolAddresses.has(pool.address);

              return (
                <div
                  key={pool.address}
                  data-index={virtualRow.index}
                  ref={virtualizer.measureElement}
                  className={clsx(
                    "absolute left-0 top-0 flex w-full cursor-pointer border-b border-white/5 text-sm text-gray-400 transition-colors duration-300 hover:bg-white/5",
                    isNew && "animate-highlight bg-green-500/10",
                  )}
                  style={{
                    height: `${ROW_HEIGHT}px`,
                    transform: `translateY(${virtualRow.start}px)`,
                  }}
                  onClick={() => router.push(`/pair/${pool.address}`)}
                >
                  <div className="flex w-1/4 items-center px-6 font-mono text-blue-400">
                    {pool.address.slice(0, 6)}...{pool.address.slice(-4)}
                  </div>
                  <div className="flex w-1/4 items-center px-6 font-mono text-purple-400">
                    {pool.mint.slice(0, 6)}...{pool.mint.slice(-4)}
                  </div>
                  <div className="flex w-1/4 items-center px-6 font-mono text-white">
                    {(Number(pool.solAmount) / 1e9).toFixed(2)} SOL
                  </div>
                  <div className="flex w-1/4 items-center justify-end px-6 font-mono text-gray-500">
                    {new Date(pool.timestamp).toLocaleTimeString()}
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
};
