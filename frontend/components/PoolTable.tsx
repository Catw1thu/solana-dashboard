"use client";

import { useState, useEffect } from "react";
import { useSocket } from "../context/SocketContext";
import { useSocketSubscription } from "../hooks/useSocketSubscription";
import { PoolData } from "../types";
import { motion, AnimatePresence } from "framer-motion";
import { Play } from "lucide-react";
import clsx from "clsx";
import { useRouter } from "next/navigation";

export const PoolTable = () => {
  const { socket } = useSocket();
  const router = useRouter();
  const [pools, setPools] = useState<PoolData[]>([]);

  // 1. Subscribe to Global Room
  useSocketSubscription("room:global");

  // 2. Listen for new pools
  useEffect(() => {
    if (!socket) return;

    const handleNewPool = (pool: PoolData) => {
      console.log("New Pool:", pool);
      setPools((prev) => [pool, ...prev].slice(0, 50)); // Keep last 50
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
          "http://localhost:3000/api/token/pools?limit=20"
        );
        const data = await res.json();

        // Map API response to PoolData interface
        // Note: Historical API data might lack initial liquidity amounts (solAmount/tokenAmount)
        const mappedPools: PoolData[] = data.map((p: any) => ({
          address: p.address,
          mint: p.baseMint,
          solAmount: "0", // Not available in DB history
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

  return (
    <div className="w-full overflow-hidden rounded-xl border border-white/10 bg-white/5 backdrop-blur-md">
      <div className="flex items-center justify-between border-b border-white/10 px-6 py-4">
        <h2 className="flex items-center gap-2 text-lg font-semibold text-white">
          <Play className="h-5 w-5 fill-green-500 text-green-500" />
          Live New Pools
        </h2>
        <span className="flex h-2 w-2 animate-pulse rounded-full bg-green-500 shadow-[0_0_10px_#22c55e]"></span>
      </div>

      <div className="w-full overflow-x-auto">
        <table className="w-full text-left text-sm text-gray-400">
          <thead className="bg-white/5 text-xs uppercase text-gray-200">
            <tr>
              <th className="px-6 py-3 font-medium">Pool Address</th>
              <th className="px-6 py-3 font-medium">Token Mint</th>
              <th className="px-6 py-3 font-medium">Initial SOL</th>
              <th className="px-6 py-3 font-medium text-right">Time</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-white/5">
            <AnimatePresence initial={false}>
              {pools.map((pool) => (
                <motion.tr
                  key={pool.address}
                  initial={{
                    opacity: 0,
                    x: -20,
                    backgroundColor: "rgba(34, 197, 94, 0.1)",
                  }}
                  animate={{ opacity: 1, x: 0, backgroundColor: "transparent" }}
                  exit={{ opacity: 0 }}
                  transition={{ duration: 0.3 }}
                  onClick={() => router.push(`/pair/${pool.address}`)}
                  className="group hover:bg-white/5 cursor-pointer"
                >
                  <td className="px-6 py-4 font-mono text-blue-400">
                    {pool.address.slice(0, 6)}...{pool.address.slice(-4)}
                  </td>
                  <td className="px-6 py-4 font-mono text-purple-400">
                    {pool.mint.slice(0, 6)}...{pool.mint.slice(-4)}
                  </td>
                  <td className="px-6 py-4 font-mono text-white">
                    {(Number(pool.solAmount) / 1e9).toFixed(2)} SOL
                  </td>
                  <td className="px-6 py-4 text-right font-mono text-gray-500">
                    {new Date(pool.timestamp).toLocaleTimeString()}
                  </td>
                </motion.tr>
              ))}
            </AnimatePresence>
            {pools.length === 0 && (
              <tr>
                <td colSpan={4} className="px-6 py-8 text-center text-gray-600">
                  Waiting for new pools...
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
};
