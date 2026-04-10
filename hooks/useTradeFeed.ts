import { useState, useEffect, useCallback, useRef } from "react";
import { TokenCandle } from "../types";
import { API } from "../config/api";

export type Resolution = "1m" | "5m" | "15m" | "1h" | "4h" | "1d";

const INITIAL_CANDLE_PAGE_SIZE = 300;
const HISTORY_CANDLE_PAGE_SIZE = 200;

function mergeCandlePages(
  olderCandles: TokenCandle[],
  currentCandles: TokenCandle[],
): TokenCandle[] {
  const merged = new Map<number, TokenCandle>();
  for (const candle of [...olderCandles, ...currentCandles]) {
    merged.set(candle.time, candle);
  }
  return [...merged.values()].sort((a, b) => a.time - b.time);
}

export const useTradeFeed = (mint: string) => {
  const [candles, setCandles] = useState<TokenCandle[]>([]);
  const [resolution, setResolution] = useState<Resolution>("1m");
  const [isHistoryLoaded, setIsHistoryLoaded] = useState(false);
  const [hasMoreHistory, setHasMoreHistory] = useState(true);
  const [isLoadingMoreHistory, setIsLoadingMoreHistory] = useState(false);
  const generationRef = useRef(0);

  const fetchCandlesPage = useCallback(
    async (limit: number, beforeTime?: number) => {
      const candlesRes = await fetch(
        API.tokenCandles(mint, resolution, limit, beforeTime),
      );

      if (!candlesRes.ok) {
        throw new Error(`failed to fetch candles: ${candlesRes.status}`);
      }

      const candlesData = await candlesRes.json();
      return (candlesData.candles || []) as TokenCandle[];
    },
    [mint, resolution],
  );

  const reload = useCallback(async () => {
    if (!mint) return;

    const generation = generationRef.current + 1;
    generationRef.current = generation;
    setIsHistoryLoaded(false);
    setHasMoreHistory(true);
    setIsLoadingMoreHistory(false);
    try {
      const nextCandles = await fetchCandlesPage(INITIAL_CANDLE_PAGE_SIZE);
      if (generationRef.current !== generation) {
        return;
      }
      setCandles(nextCandles);
      setHasMoreHistory(nextCandles.length >= INITIAL_CANDLE_PAGE_SIZE);
    } catch (e) {
      if (generationRef.current !== generation) {
        return;
      }
      console.error("Failed to fetch history", e);
      setCandles([]);
      setHasMoreHistory(false);
    } finally {
      if (generationRef.current === generation) {
        setIsHistoryLoaded(true);
      }
    }
  }, [fetchCandlesPage, mint]);

  const loadMoreHistory = useCallback(async () => {
    if (
      !mint ||
      isLoadingMoreHistory ||
      !hasMoreHistory ||
      candles.length === 0
    ) {
      return;
    }

    setIsLoadingMoreHistory(true);
    const generation = generationRef.current;
    try {
      const nextChunk = await fetchCandlesPage(
        HISTORY_CANDLE_PAGE_SIZE,
        candles[0].time,
      );
      if (generationRef.current !== generation) {
        return;
      }
      setCandles((current) => mergeCandlePages(nextChunk, current));
      setHasMoreHistory(nextChunk.length >= HISTORY_CANDLE_PAGE_SIZE);
    } catch (e) {
      if (generationRef.current !== generation) {
        return;
      }
      console.error("Failed to load older candles", e);
    } finally {
      if (generationRef.current === generation) {
        setIsLoadingMoreHistory(false);
      }
    }
  }, [
    candles,
    fetchCandlesPage,
    hasMoreHistory,
    isLoadingMoreHistory,
    mint,
  ]);

  useEffect(() => {
    if (!mint) {
      generationRef.current += 1;
      setCandles([]);
      setHasMoreHistory(false);
      setIsHistoryLoaded(false);
      setIsLoadingMoreHistory(false);
      return;
    }

    reload();
  }, [mint, resolution, reload]);

  return {
    candles,
    setCandles,
    resolution,
    setResolution,
    isHistoryLoaded,
    hasMoreHistory,
    isLoadingMoreHistory,
    loadMoreHistory,
    reload,
  };
};
