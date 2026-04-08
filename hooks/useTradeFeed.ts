import { useState, useEffect, useCallback } from "react";
import { TokenCandle } from "../types";
import { API } from "../config/api";

export type Resolution = "1m" | "5m" | "15m" | "1h" | "4h" | "1d";

export const useTradeFeed = (mint: string) => {
  const [candles, setCandles] = useState<TokenCandle[]>([]);
  const [resolution, setResolution] = useState<Resolution>("1m");
  const [isHistoryLoaded, setIsHistoryLoaded] = useState(false);

  const reload = useCallback(async () => {
    if (!mint) return;

    setIsHistoryLoaded(false);
    try {
      const candlesRes = await fetch(API.tokenCandles(mint, resolution));

      if (candlesRes.ok) {
        const candlesData = await candlesRes.json();
        setCandles(candlesData.candles || []);
      }
    } catch (e) {
      console.error("Failed to fetch history", e);
    } finally {
      setIsHistoryLoaded(true);
    }
  }, [mint, resolution]);

  useEffect(() => {
    if (!mint) return;

    // Initial fetch
    reload();

    // Polling disabled for now
    // const interval = setInterval(fetchData, 10000);
    // return () => clearInterval(interval);
  }, [mint, resolution, reload]);

  return {
    candles,
    setCandles,
    resolution,
    setResolution,
    isHistoryLoaded,
    reload,
  };
};
