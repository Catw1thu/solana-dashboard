"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import { useSocket } from "../context/SocketContext";
import { useSocketSubscription } from "../hooks/useSocketSubscription";
import { PoolData } from "../types";
import { Play, TrendingUp, TrendingDown } from "lucide-react";
import clsx from "clsx";
import { useRouter } from "next/navigation";
import { useVirtualizer } from "@tanstack/react-virtual";
import { API } from "../config/api";
import { formatPrice } from "../utils/format";

const ROW_HEIGHT = 52;
const MAX_POOLS = 500;
const PAGE_SIZE = 20;

export const PoolTable = () => {
  const { socket } = useSocket();
  const router = useRouter();
  const [pools, setPools] = useState<PoolData[]>([]);
  const [newPoolAddresses, setNewPoolAddresses] = useState<Set<string>>(
    new Set(),
  );
  const [containerHeight, setContainerHeight] = useState(400);
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(true);
  const parentRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const updateHeight = () => {
      if (parentRef.current?.parentElement) {
        const parent = parentRef.current.parentElement;
        const headerHeight =
          parent
            .querySelector(".flex.items-center.justify-between")
            ?.getBoundingClientRect().height || 60;
        const tableHeaderHeight =
          parent.querySelector(".table-header")?.getBoundingClientRect()
            .height || 44;
        const availableHeight =
          parent.getBoundingClientRect().height -
          headerHeight -
          tableHeaderHeight;
        setContainerHeight(Math.max(200, availableHeight));
      }
    };
    updateHeight();
    window.addEventListener("resize", updateHeight);
    const timer = setTimeout(updateHeight, 100);
    return () => {
      window.removeEventListener("resize", updateHeight);
      clearTimeout(timer);
    };
  }, []);

  useSocketSubscription("room:global");

  useEffect(() => {
    if (!socket) return;

    const handleNewPool = (pool: PoolData) => {
      setNewPoolAddresses((prev) => new Set(prev).add(pool.address));
      setTimeout(() => {
        setNewPoolAddresses((prev) => {
          const next = new Set(prev);
          next.delete(pool.address);
          return next;
        });
      }, 1000);

      setPools((prev) => [pool, ...prev].slice(0, MAX_POOLS));

      // Fetch metadata + stats after delay
      setTimeout(async () => {
        try {
          const res = await fetch(API.pool(pool.address));
          if (!res.ok) return;
          const data = await res.json();
          if (data) {
            setPools((prev) =>
              prev.map((p) =>
                p.address === pool.address
                  ? {
                      ...p,
                      name: data.name,
                      symbol: data.symbol,
                      image: data.image,
                    }
                  : p,
              ),
            );
          }
        } catch {
          /* ignore */
        }
      }, 3000);
    };

    socket.on("pool:new", handleNewPool);
    return () => {
      socket.off("pool:new", handleNewPool);
    };
  }, [socket]);

  const mapPoolData = (p: any): PoolData => ({
    address: p.address,
    mint: p.baseMint,
    solAmount: "0",
    tokenAmount: "0",
    timestamp: new Date(p.createdAt).getTime(),
    name: p.name || undefined,
    symbol: p.symbol || undefined,
    image: p.image || undefined,
    price: p.price,
    priceChange5m: p.priceChange5m,
    volume5m: p.volume5m,
    txns5m: p.txns5m,
    buys5m: p.buys5m,
    sells5m: p.sells5m,
    liquidity: p.liquidity,
    mcap: p.mcap,
  });

  // Fetch initial data with stats
  useEffect(() => {
    const fetchPools = async () => {
      try {
        const res = await fetch(
          `${API.poolsStats}?limit=${PAGE_SIZE}&offset=0`,
        );
        const data = await res.json();
        setPools(data.map(mapPoolData));
        setHasMore(data.length >= PAGE_SIZE);
      } catch (err) {
        console.error("Failed to fetch initial pools:", err);
      }
    };
    fetchPools();
  }, []);

  // Load more pools when scrolling near bottom
  const loadMore = useCallback(async () => {
    if (isLoadingMore || !hasMore) return;
    setIsLoadingMore(true);
    try {
      const offset = pools.length;
      const res = await fetch(
        `${API.poolsStats}?limit=${PAGE_SIZE}&offset=${offset}`,
      );
      const data = await res.json();
      if (data.length === 0) {
        setHasMore(false);
      } else {
        setPools((prev) => {
          // Deduplicate by address
          const existing = new Set(prev.map((p) => p.address));
          const newPools = data
            .map(mapPoolData)
            .filter((p: PoolData) => !existing.has(p.address));
          return [...prev, ...newPools].slice(0, MAX_POOLS);
        });
        setHasMore(data.length >= PAGE_SIZE);
      }
    } catch (err) {
      console.error("Failed to load more pools:", err);
    } finally {
      setIsLoadingMore(false);
    }
  }, [isLoadingMore, hasMore, pools.length]);

  // Detect scroll near bottom — trigger loadMore
  useEffect(() => {
    const el = parentRef.current;
    if (!el) return;
    const handleScroll = () => {
      const nearBottom =
        el.scrollHeight - el.scrollTop - el.clientHeight < ROW_HEIGHT * 3;
      if (nearBottom) loadMore();
    };
    el.addEventListener("scroll", handleScroll, { passive: true });
    return () => el.removeEventListener("scroll", handleScroll);
  }, [loadMore]);

  const virtualizer = useVirtualizer({
    count: pools.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 5,
  });

  const virtualItems = virtualizer.getVirtualItems();

  const formatAge = (timestamp: number) => {
    const diff = Date.now() - timestamp;
    const seconds = Math.floor(diff / 1000);
    if (seconds < 60) return `${seconds}s`;
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return `${minutes}m`;
    const hours = Math.floor(minutes / 60);
    if (hours < 24) return `${hours}h`;
    const days = Math.floor(hours / 24);
    return `${days}d`;
  };

  const formatVolume = (vol: number) => {
    if (vol >= 1_000_000) return `${(vol / 1_000_000).toFixed(1)}M`;
    if (vol >= 1000) return `${(vol / 1000).toFixed(1)}K`;
    if (vol >= 1) return vol.toFixed(1);
    return vol.toFixed(2);
  };

  return (
    <div className="flex-1 flex flex-col overflow-hidden rounded-xl border border-(--border-primary) bg-(--bg-secondary)">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-(--border-primary) px-6 py-4">
        <h2 className="flex items-center gap-2 text-lg font-semibold text-(--text-primary)">
          <Play className="h-5 w-5 fill-(--accent-green) text-(--accent-green)" />
          Live New Pools
          <span className="ml-2 text-sm font-normal text-(--text-muted)">
            ({pools.length})
          </span>
        </h2>
        <span className="live-dot" />
      </div>

      {/* Table Header — 8 columns */}
      <div className="table-header grid grid-cols-[2fr_1fr_0.8fr_0.8fr_0.8fr_0.8fr_0.6fr_0.5fr] px-4">
        <div className="px-3 py-3">Token</div>
        <div className="px-3 py-3">Price</div>
        <div className="px-3 py-3">5m %</div>
        <div className="px-3 py-3">Liq</div>
        <div className="px-3 py-3">MCap</div>
        <div className="px-3 py-3">Vol (5m)</div>
        <div className="px-3 py-3">Txns</div>
        <div className="px-3 py-3 text-right">Age</div>
      </div>

      {/* Virtualized Body */}
      <div
        ref={parentRef}
        className="overflow-auto scrollbar-thin"
        style={{ height: containerHeight }}
      >
        {pools.length === 0 ? (
          <div className="px-6 py-8 text-center text-(--text-disabled)">
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
              const change = pool.priceChange5m;
              const isUp = change != null && change > 0;
              const isDown = change != null && change < 0;
              const isFlat = change != null && change === 0;

              return (
                <div
                  key={pool.address}
                  data-index={virtualRow.index}
                  ref={virtualizer.measureElement}
                  className={clsx(
                    "absolute left-0 top-0 grid grid-cols-[2fr_1fr_0.8fr_0.8fr_0.8fr_0.8fr_0.6fr_0.5fr] w-full cursor-pointer border-b border-(--border-primary)/50 text-sm text-(--text-secondary) transition-colors duration-200",
                    "hover:bg-(--bg-tertiary)",
                    isNew && "animate-highlight",
                  )}
                  style={{
                    height: `${ROW_HEIGHT}px`,
                    transform: `translateY(${virtualRow.start}px)`,
                  }}
                  onClick={() => router.push(`/pair/${pool.address}`)}
                >
                  {/* Token */}
                  <div className="flex items-center gap-2 px-3">
                    {pool.image ? (
                      <img
                        src={pool.image}
                        alt={pool.symbol || "token"}
                        className="w-7 h-7 rounded-full object-cover shrink-0 ring-1 ring-(--border-primary)"
                        onError={(e) => {
                          (e.target as HTMLImageElement).style.display = "none";
                        }}
                      />
                    ) : (
                      <div className="w-7 h-7 rounded-full bg-(--bg-tertiary) shrink-0 ring-1 ring-(--border-primary) flex items-center justify-center text-xs text-(--text-muted) font-bold">
                        {pool.symbol ? pool.symbol[0] : "?"}
                      </div>
                    )}
                    <div className="flex flex-col min-w-0">
                      <span className="font-semibold text-(--text-primary) truncate text-sm">
                        {pool.symbol || `${pool.mint.slice(0, 6)}...`}
                      </span>
                      {pool.name && (
                        <span className="text-[11px] text-(--text-muted) truncate leading-tight">
                          {pool.name}
                        </span>
                      )}
                    </div>
                  </div>

                  {/* Price */}
                  <div className="flex items-center px-3 font-mono text-(--text-primary) text-xs">
                    {pool.price ? formatPrice(pool.price) : "--"}
                  </div>

                  {/* 5m Change */}
                  <div className="flex items-center gap-1 px-3">
                    {change != null ? (
                      <>
                        {isUp && (
                          <TrendingUp
                            size={13}
                            className="text-(--accent-green) shrink-0"
                          />
                        )}
                        {isDown && (
                          <TrendingDown
                            size={13}
                            className="text-(--accent-red) shrink-0"
                          />
                        )}
                        <span
                          className={clsx(
                            "font-mono text-xs font-semibold",
                            isUp
                              ? "text-(--accent-green)"
                              : isDown
                                ? "text-(--accent-red)"
                                : "text-(--text-muted)",
                          )}
                        >
                          {change >= 0 ? "+" : ""}
                          {change.toFixed(1)}%
                        </span>
                      </>
                    ) : (
                      <span className="text-(--text-muted) text-xs">--</span>
                    )}
                  </div>

                  {/* Liquidity */}
                  <div className="flex items-center px-3 font-mono text-xs text-(--text-secondary)">
                    {pool.liquidity != null
                      ? `${formatVolume(pool.liquidity)} SOL`
                      : "--"}
                  </div>

                  {/* MCap */}
                  <div className="flex items-center px-3 font-mono text-xs text-(--text-secondary)">
                    {pool.mcap != null
                      ? `${formatVolume(pool.mcap)} SOL`
                      : "--"}
                  </div>

                  {/* Volume 5m */}
                  <div className="flex items-center px-3 font-mono text-xs text-(--text-secondary)">
                    {pool.volume5m != null
                      ? `${formatVolume(pool.volume5m)} SOL`
                      : "--"}
                  </div>

                  {/* Txns 5m */}
                  <div className="flex items-center px-3 font-mono text-xs text-(--text-secondary)">
                    {pool.txns5m != null ? pool.txns5m : "--"}
                  </div>

                  {/* Age */}
                  <div className="flex items-center justify-end px-3 font-mono text-xs text-(--text-muted)">
                    {formatAge(pool.timestamp)}
                  </div>
                </div>
              );
            })}
          </div>
        )}
        {isLoadingMore && (
          <div className="flex items-center justify-center py-3 text-sm text-(--text-muted)">
            <svg className="animate-spin h-4 w-4 mr-2" viewBox="0 0 24 24">
              <circle
                className="opacity-25"
                cx="12"
                cy="12"
                r="10"
                stroke="currentColor"
                strokeWidth="4"
                fill="none"
              />
              <path
                className="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
              />
            </svg>
            Loading more...
          </div>
        )}
      </div>
    </div>
  );
};
