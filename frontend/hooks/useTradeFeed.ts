import { useState, useEffect, useRef, useCallback } from "react";
import { useSocket } from "../context/SocketContext";
import { useSocketSubscription } from "./useSocketSubscription";
import { Trade } from "../types";
import { API } from "../config/api";

export interface CandleData {
  time: number; // Unix timestamp in ms
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
}

export type Resolution = "1m" | "5m" | "15m" | "1h" | "4h" | "1d";

// Resolution to milliseconds mapping
const RESOLUTION_MS: Record<Resolution, number> = {
  "1m": 60 * 1000,
  "5m": 5 * 60 * 1000,
  "15m": 15 * 60 * 1000,
  "1h": 60 * 60 * 1000,
  "4h": 4 * 60 * 60 * 1000,
  "1d": 24 * 60 * 60 * 1000,
};

/**
 * Aggregate a single trade into an array of candles.
 * Mutates the candles array in place for performance.
 */
function aggregateTradeToCandles(
  candles: CandleData[],
  trade: Trade,
  resolutionMs: number,
): void {
  const tradeTime =
    typeof trade.time === "number"
      ? trade.time
      : new Date(trade.time).getTime();
  const bucketTime = Math.floor(tradeTime / resolutionMs) * resolutionMs;

  if (candles.length === 0) {
    // No candles yet, create first one
    candles.push({
      time: bucketTime,
      open: trade.price,
      high: trade.price,
      low: trade.price,
      close: trade.price,
      volume: trade.baseAmount,
    });
    return;
  }

  const lastCandle = candles[candles.length - 1];

  if (lastCandle.time === bucketTime) {
    // Trade belongs to current candle - update OHLC
    lastCandle.high = Math.max(lastCandle.high, trade.price);
    lastCandle.low = Math.min(lastCandle.low, trade.price);
    lastCandle.close = trade.price;
    lastCandle.volume += trade.baseAmount;
  } else if (bucketTime > lastCandle.time) {
    // Trade is in a newer bucket - create new candle
    candles.push({
      time: bucketTime,
      open: trade.price,
      high: trade.price,
      low: trade.price,
      close: trade.price,
      volume: trade.baseAmount,
    });
  }
  // If bucketTime < lastCandle.time, it's an out-of-order trade (ignore for real-time)
}

export const useTradeFeed = (poolAddress: string) => {
  const { socket } = useSocket();
  const [trades, setTrades] = useState<Trade[]>([]);
  const [candles, setCandles] = useState<CandleData[]>([]);
  const [resolution, setResolution] = useState<Resolution>("1m");
  const [isHistoryLoaded, setIsHistoryLoaded] = useState(false);

  // Keep a mutable ref for candles to avoid stale closures
  const candlesRef = useRef<CandleData[]>([]);

  // 1. Subscribe to the room
  useSocketSubscription(poolAddress ? `room:${poolAddress}` : "");

  // 2. Load historical candles and trades when address or resolution changes
  useEffect(() => {
    if (!poolAddress) return;

    setIsHistoryLoaded(false);
    candlesRef.current = [];

    const fetchHistory = async () => {
      try {
        // Fetch historical candles and trades in parallel
        const [candlesRes, tradesRes] = await Promise.all([
          fetch(API.candles(poolAddress, resolution)),
          fetch(API.trades(poolAddress, 100)),
        ]);

        const candlesData = await candlesRes.json();
        const tradesData = await tradesRes.json();

        // Format candles
        const formattedCandles: CandleData[] = candlesData.map((c: any) => ({
          time: new Date(c.time).getTime(),
          open: c.open,
          high: c.high,
          low: c.low,
          close: c.close,
          volume: c.volume,
        }));
        formattedCandles.sort((a, b) => a.time - b.time);

        // Format trades (already sorted by time desc from backend)
        const formattedTrades: Trade[] = tradesData.map((t: any) => ({
          ...t,
          time: new Date(t.time).getTime(),
        }));

        candlesRef.current = formattedCandles;
        setCandles(formattedCandles);
        setTrades(formattedTrades);
        setIsHistoryLoaded(true);
      } catch (e) {
        console.error("Failed to fetch history", e);
        setIsHistoryLoaded(true); // Allow real-time updates even if history fails
      }
    };

    fetchHistory();
  }, [poolAddress, resolution]);

  // 3. Listen for batch events and aggregate to candles
  useEffect(() => {
    if (!socket || !poolAddress) return;

    const resolutionMs = RESOLUTION_MS[resolution];

    const handleBatch = (batch: Trade[]) => {
      // Update trades list (newest first)
      setTrades((prev) => [...batch, ...prev].slice(0, 1000));

      // Only aggregate if history is loaded
      if (!isHistoryLoaded) return;

      // Aggregate each trade to candles (oldest first for correct OHLC)
      const sortedBatch = [...batch].sort((a, b) => {
        const timeA =
          typeof a.time === "number" ? a.time : new Date(a.time).getTime();
        const timeB =
          typeof b.time === "number" ? b.time : new Date(b.time).getTime();
        return timeA - timeB;
      });

      for (const trade of sortedBatch) {
        aggregateTradeToCandles(candlesRef.current, trade, resolutionMs);
      }

      // Trigger re-render with new candles array reference
      setCandles([...candlesRef.current]);
    };

    socket.on("trade:batch", handleBatch);

    return () => {
      socket.off("trade:batch", handleBatch);
    };
  }, [socket, poolAddress, resolution, isHistoryLoaded]);

  return {
    trades,
    candles,
    resolution,
    setResolution,
  };
};
