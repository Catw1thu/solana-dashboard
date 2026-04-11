"use client";

import { useParams } from "next/navigation";
import { useTradeFeed } from "@/hooks/useTradeFeed";
import { TradingChart, type ChartMarkerData } from "@/components/TradingChart";
import { ExternalLink, Copy, Check } from "lucide-react";
import { formatAddress, formatPrice, formatAmount } from "@/utils/format";
import {
  startTransition,
  type ReactNode,
  useSyncExternalStore,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import clsx from "clsx";
import {
  TokenActivity,
  TokenCandle,
  TokenActivityCursor,
  TokenActivityPage,
  TokenDetail,
  TokenEventEnvelope,
  TokenStat,
} from "@/types";
import { API } from "@/config/api";
import { useAppWebSocket } from "@/context/WebSocketContext";
import {
  applyTradePatchToCandles,
  buildRealtimeActivity,
  buildRealtimeCandlePatch,
  type CandleResolution,
  eventBelongsToMint,
} from "@/utils/tokenRealtime";

const TRADE_ROW_HEIGHT = 36;
const WS_REFRESH_COOLDOWN_MS = 1200;
const ACTIVITY_PAGE_SIZE = 50;
const ACTIVITY_TOP_THRESHOLD = 8;
const ACTIVITY_LOAD_MORE_THRESHOLD = 240;
const ACTIVITY_VIRTUAL_OVERSCAN = 12;

type WsMessage = {
  log_id?: number;
  event_type?: string;
  refs?: {
    mint?: string | null;
  };
  payload?: unknown;
  [key: string]: unknown;
};

type MetricTone = "positive" | "negative" | "neutral";
type StatsWindow = "1m" | "5m" | "1h" | "4h" | "24h";

function resolutionBucketSeconds(resolution: CandleResolution | "1d"): number {
  switch (resolution) {
    case "1m":
      return 60;
    case "5m":
      return 300;
    case "15m":
      return 900;
    case "1h":
      return 3600;
    case "4h":
      return 14400;
    case "1d":
      return 86400;
    default:
      return 60;
  }
}

function bucketUnixTime(
  eventUnixTs: number,
  resolution: CandleResolution | "1d",
): number {
  const seconds =
    eventUnixTs > 1e11 ? Math.floor(eventUnixTs / 1000) : eventUnixTs;
  const bucket = resolutionBucketSeconds(resolution);
  return Math.floor(seconds / bucket) * bucket;
}

function formatMarkerDateTime(eventUnixTs: number): string {
  const ms = eventUnixTs > 1e11 ? eventUnixTs : eventUnixTs * 1000;
  return new Date(ms).toLocaleString("zh-CN");
}

let relativeClockNowMs = Date.now();
let relativeClockTimer: ReturnType<typeof setInterval> | null = null;
const relativeClockListeners = new Set<() => void>();

function emitRelativeClock() {
  relativeClockNowMs = Date.now();
  for (const listener of relativeClockListeners) {
    listener();
  }
}

function subscribeRelativeClock(listener: () => void) {
  relativeClockListeners.add(listener);
  if (relativeClockTimer == null) {
    relativeClockTimer = setInterval(emitRelativeClock, 1_000);
  }

  return () => {
    relativeClockListeners.delete(listener);
    if (relativeClockListeners.size === 0 && relativeClockTimer != null) {
      clearInterval(relativeClockTimer);
      relativeClockTimer = null;
    }
  };
}

function getRelativeClockSnapshot() {
  return relativeClockNowMs;
}

function formatRelativeAge(unixTs: number, nowMs: number): string {
  if (!unixTs) return "--";
  const ms = unixTs > 1e11 ? unixTs : unixTs * 1000;
  const diff = Math.max(0, nowMs - ms);
  const s = Math.floor(diff / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h`;
  return `${Math.floor(h / 24)}d`;
}

function RelativeAgeText({
  unixTs,
  title,
  className,
}: {
  unixTs: number | null | undefined;
  title?: string;
  className?: string;
}) {
  const nowMs = useSyncExternalStore(
    subscribeRelativeClock,
    getRelativeClockSnapshot,
    getRelativeClockSnapshot,
  );

  if (!unixTs) {
    return <span className={className}>--</span>;
  }

  return (
    <span className={className} title={title}>
      {formatRelativeAge(unixTs, nowMs)}
    </span>
  );
}

function metricTone(value?: number | null): MetricTone {
  if (value == null || !Number.isFinite(value) || value === 0) {
    return "neutral";
  }
  return value > 0 ? "positive" : "negative";
}

function formatSignedPercent(value?: number | null): string {
  if (value == null || !Number.isFinite(value)) return "--";
  return `${value > 0 ? "+" : ""}${value.toFixed(2)}%`;
}

function formatCompactValue(
  value: number | null | undefined,
  signed = false,
): string {
  if (value == null || !Number.isFinite(value)) return "--";
  const abs = Math.abs(value);
  const sign = signed ? (value > 0 ? "+" : value < 0 ? "-" : "") : "";
  return `${sign}${formatAmount(abs)} SOL`;
}

function formatQuoteValue(
  value: number | null | undefined,
  signed = false,
): string {
  if (value == null || !Number.isFinite(value)) return "--";
  return `${formatCompactValue(value, signed)}`;
}

function formatCountAndValue(
  count: number | null | undefined,
  amount: number | null | undefined,
): string {
  if (count == null && amount == null) return "--";
  const safeCount = count ?? 0;
  if (amount == null || !Number.isFinite(amount)) {
    return safeCount.toLocaleString();
  }
  return `${safeCount.toLocaleString()} / ${formatAmount(Math.abs(amount))} SOL`;
}

function humanizeLabel(value?: string | null): string {
  if (!value) return "--";
  return value
    .split(/[_\s]+/)
    .filter(Boolean)
    .map((part) => part[0].toUpperCase() + part.slice(1))
    .join(" ");
}

function scaleRawAmount(
  raw: string | null | undefined,
  decimals: number | null | undefined,
): number | null {
  if (!raw) return null;
  const numeric = Number(raw);
  if (!Number.isFinite(numeric)) return null;
  const scale = decimals ?? 0;
  return numeric / 10 ** scale;
}

function readPayloadString(
  payload: Record<string, unknown> | undefined,
  key: string,
): string | null {
  const value = payload?.[key];
  return typeof value === "string" ? value : null;
}

function resolveQuoteSymbol(detail: TokenDetail | null): string {
  return detail?.quote?.symbol || "SOL";
}

function mergeActivities(
  current: TokenActivity[],
  incoming: TokenActivity[],
  mode: "prepend" | "append" = "prepend",
): TokenActivity[] {
  if (current.length === 0) return incoming;
  if (incoming.length === 0) return current;

  const seen = new Set<string>();
  const merged: TokenActivity[] = [];
  const ordered =
    mode === "append" ? [...current, ...incoming] : [...incoming, ...current];
  for (const item of ordered) {
    if (seen.has(item.event_id)) continue;
    seen.add(item.event_id);
    merged.push(item);
  }
  return merged;
}

function mergeBufferedEnvelopes(
  current: TokenEventEnvelope[],
  incoming: TokenEventEnvelope[],
): TokenEventEnvelope[] {
  if (incoming.length === 0) return current;

  const seen = new Set<string>();
  const merged: TokenEventEnvelope[] = [];
  for (const item of [...current, ...incoming]) {
    if (seen.has(item.event_id)) continue;
    seen.add(item.event_id);
    merged.push(item);
  }
  return merged;
}

function computeMarketCapQuote(
  detail: TokenDetail | null,
  latestPrice: number | null | undefined,
): number | null {
  if (latestPrice == null || !Number.isFinite(latestPrice)) return null;

  const supplyRaw =
    detail?.total_supply_raw ||
    detail?.create_event?.token_total_supply ||
    null;
  const decimals = detail?.decimals ?? detail?.create_event?.decimals ?? 0;
  const supply = scaleRawAmount(supplyRaw, decimals);
  if (supply == null) return null;

  return supply * latestPrice;
}

function computeLiquidityQuote(
  detail: TokenDetail | null,
  latestPrice: number | null | undefined,
): number | null {
  if (!detail) return null;

  const recentEvents: TokenEventEnvelope[] = detail.recent_events || [];
  const activeMarket = detail.active_market;
  const tokenDecimals =
    detail.decimals ?? activeMarket?.base_mint_decimals ?? 6;

  for (const event of recentEvents) {
    const payload = event.payload;
    if (!payload || typeof payload !== "object") continue;

    if (event.protocol === "pumpfun") {
      const solReserve = scaleRawAmount(
        readPayloadString(payload, "real_sol_reserves") ||
          readPayloadString(payload, "virtual_sol_reserves"),
        9,
      );
      const tokenReserve = scaleRawAmount(
        readPayloadString(payload, "real_token_reserves") ||
          readPayloadString(payload, "virtual_token_reserves"),
        tokenDecimals,
      );

      if (solReserve == null) continue;
      if (tokenReserve != null && latestPrice != null) {
        return solReserve + tokenReserve * latestPrice;
      }
      return solReserve;
    }

    if (event.protocol !== "pumpamm") continue;

    const baseMint =
      readPayloadString(payload, "base_mint") ||
      activeMarket?.base_mint ||
      null;
    const quoteMint =
      readPayloadString(payload, "quote_mint") ||
      activeMarket?.quote_mint ||
      null;
    if (!baseMint || !quoteMint) continue;

    let tokenReserveRaw: string | null = null;
    let quoteReserveRaw: string | null = null;
    let quoteDecimals: number | null | undefined = null;

    if (baseMint === detail.mint) {
      tokenReserveRaw =
        readPayloadString(payload, "pool_base_token_reserves") ||
        readPayloadString(payload, "base_amount_in");
      quoteReserveRaw =
        readPayloadString(payload, "pool_quote_token_reserves") ||
        readPayloadString(payload, "quote_amount_in");
      quoteDecimals =
        activeMarket?.quote_mint_decimals ?? detail.quote?.decimals ?? 9;
    } else if (quoteMint === detail.mint) {
      tokenReserveRaw =
        readPayloadString(payload, "pool_quote_token_reserves") ||
        readPayloadString(payload, "quote_amount_in");
      quoteReserveRaw =
        readPayloadString(payload, "pool_base_token_reserves") ||
        readPayloadString(payload, "base_amount_in");
      quoteDecimals =
        activeMarket?.base_mint_decimals ??
        (activeMarket?.base_mint ===
        "So11111111111111111111111111111111111111112"
          ? 9
          : (detail.quote?.decimals ?? 9));
    }

    const tokenReserve = scaleRawAmount(tokenReserveRaw, tokenDecimals);
    const quoteReserve = scaleRawAmount(quoteReserveRaw, quoteDecimals);
    if (quoteReserve == null) continue;
    if (tokenReserve != null && latestPrice != null) {
      return quoteReserve + tokenReserve * latestPrice;
    }
    return quoteReserve;
  }

  return null;
}

function buildMigrateMarkers(
  detail: TokenDetail | null,
  resolution: CandleResolution | "1d",
): ChartMarkerData[] {
  const eventMap = new Map<string, TokenEventEnvelope>();
  for (const event of detail?.recent_events || []) {
    if (event.protocol === "pumpfun" && event.event_type === "migrate") {
      eventMap.set(event.event_id, event);
    }
  }
  if (detail?.migrate_event) {
    eventMap.set(detail.migrate_event.event_id, detail.migrate_event);
  }
  if (eventMap.size === 0) return [];

  const tokenDecimals = detail?.decimals ?? detail?.create_event?.decimals ?? 6;

  return Array.from(eventMap.values())
    .map((event) => {
      const payload =
        event.payload && typeof event.payload === "object"
          ? (event.payload as Record<string, unknown>)
          : undefined;
      const solAmount = scaleRawAmount(
        readPayloadString(payload, "sol_amount"),
        9,
      );
      const mintAmount = scaleRawAmount(
        readPayloadString(payload, "mint_amount"),
        tokenDecimals,
      );
      const migratePrice =
        solAmount != null && mintAmount != null && mintAmount > 0
          ? solAmount / mintAmount
          : null;
      const migrateTime = formatMarkerDateTime(event.event_unix_ts);
      const priceDisplay =
        migratePrice != null ? formatPrice(migratePrice) : "--";

      return {
        time: bucketUnixTime(event.event_unix_ts, resolution),
        text: "🚀",
        color: "#3782d0",
        position: "aboveBar" as const,
        shape: "circle" as const,
        tooltip: `${migrateTime} 迁移到 PumpSwap，迁移价格 ${priceDisplay} / SOL`,
      };
    })
    .sort((left, right) => left.time - right.time);
}

export default function TokenDetailPage() {
  const params = useParams();
  const address = params?.address as string;

  const [tokenDetail, setTokenDetail] = useState<TokenDetail | null>(null);
  const [activities, setActivities] = useState<TokenActivity[]>([]);
  const [activityNextCursor, setActivityNextCursor] =
    useState<TokenActivityCursor | null>(null);
  const [hasMoreActivities, setHasMoreActivities] = useState(false);
  const [isActivityLoading, setIsActivityLoading] = useState(false);
  const [isActivityLoadingMore, setIsActivityLoadingMore] = useState(false);
  const [pendingActivityRefresh, setPendingActivityRefresh] = useState(false);
  const [activityViewportHeight, setActivityViewportHeight] = useState(0);
  const [activityScrollTop, setActivityScrollTop] = useState(0);
  const [copiedField, setCopiedField] = useState<string | null>(null);
  const [liveStat, setLiveStat] = useState<TokenStat | null>(null);
  const [liveCandle, setLiveCandle] = useState<TokenCandle | undefined>(
    undefined,
  );
  const [selectedStatsWindow, setSelectedStatsWindow] =
    useState<StatsWindow>("1m");
  const {
    isConnected,
    subscribe,
    unsubscribe,
    setTopicCursor,
    addMessageListener,
  } = useAppWebSocket();
  const refreshTimerRef = useRef<number | null>(null);
  const lastRefreshAtRef = useRef<number>(0);
  const hasConnectedOnceRef = useRef(false);
  const wasConnectedRef = useRef(false);
  const tokenDetailRef = useRef<TokenDetail | null>(null);
  const applyRealtimeEnvelopeRef = useRef<
    (envelope: TokenEventEnvelope) => void
  >(() => {});
  const candlesRef = useRef<TokenCandle[]>([]);
  const resolutionRef = useRef<CandleResolution>("1m");
  const activityScrollRef = useRef<HTMLDivElement | null>(null);
  const activityIsAtTopRef = useRef(true);
  const activityScrollRafRef = useRef<number | null>(null);
  const activityNextCursorRef = useRef<TokenActivityCursor | null>(null);
  const hasMoreActivitiesRef = useRef(false);
  const isActivityLoadingRef = useRef(false);
  const isActivityLoadingMoreRef = useRef(false);
  const activitySnapshotLogIdRef = useRef(0);
  const activityBootstrapPendingRef = useRef(true);
  const bufferedBootstrapEnvelopesRef = useRef<TokenEventEnvelope[]>([]);
  const pendingRealtimeActivitiesRef = useRef<TokenActivity[]>([]);
  const needsActivityHeadRefreshRef = useRef(false);
  const realtimeCandlePatchRef = useRef(false);

  const {
    candles,
    setCandles,
    resolution,
    setResolution,
    isHistoryLoaded,
    hasMoreHistory,
    isLoadingMoreHistory,
    loadMoreHistory,
    reload,
  } = useTradeFeed(address);

  useEffect(() => {
    tokenDetailRef.current = tokenDetail;
  }, [tokenDetail]);

  useEffect(() => {
    candlesRef.current = candles;
    if (realtimeCandlePatchRef.current) {
      realtimeCandlePatchRef.current = false;
      return;
    }
    setLiveCandle(undefined);
  }, [candles]);

  useEffect(() => {
    resolutionRef.current = resolution as CandleResolution;
    setLiveCandle(undefined);
  }, [resolution]);

  useEffect(() => {
    if (!address) return;
    const topic = `token:${address}`;
    activityBootstrapPendingRef.current = true;
    bufferedBootstrapEnvelopesRef.current = [];
    subscribe(topic, { sinceLogId: activitySnapshotLogIdRef.current });

    return () => {
      unsubscribe(topic);
    };
  }, [address, subscribe, unsubscribe]);

  useEffect(() => {
    lastRefreshAtRef.current = 0;
    if (refreshTimerRef.current !== null) {
      window.clearTimeout(refreshTimerRef.current);
      refreshTimerRef.current = null;
    }
    activityIsAtTopRef.current = true;
    activityNextCursorRef.current = null;
    hasMoreActivitiesRef.current = false;
    isActivityLoadingRef.current = false;
    isActivityLoadingMoreRef.current = false;
    activitySnapshotLogIdRef.current = 0;
    activityBootstrapPendingRef.current = true;
    bufferedBootstrapEnvelopesRef.current = [];
    pendingRealtimeActivitiesRef.current = [];
    needsActivityHeadRefreshRef.current = false;
    realtimeCandlePatchRef.current = false;
    setActivities([]);
    setActivityNextCursor(null);
    setHasMoreActivities(false);
    setPendingActivityRefresh(false);
    setActivityScrollTop(0);
    setLiveCandle(undefined);
    if (activityScrollRef.current) {
      activityScrollRef.current.scrollTop = 0;
    }
  }, [address]);

  useEffect(() => {
    activityNextCursorRef.current = activityNextCursor;
  }, [activityNextCursor]);

  useEffect(() => {
    hasMoreActivitiesRef.current = hasMoreActivities;
  }, [hasMoreActivities]);

  useEffect(() => {
    isActivityLoadingRef.current = isActivityLoading;
  }, [isActivityLoading]);

  useEffect(() => {
    isActivityLoadingMoreRef.current = isActivityLoadingMore;
  }, [isActivityLoadingMore]);

  const fetchTokenDetail = useCallback(async () => {
    if (!address) return;
    try {
      const res = await fetch(API.tokenDetail(address));
      if (res.ok) {
        setTokenDetail(await res.json());
      }
    } catch (e) {
      console.error("Failed to fetch token detail:", e);
    }
  }, [address]);

  const fetchTokenActivityPage = useCallback(
    async (mode: "reset" | "append" | "refresh_head") => {
      if (!address) return;

      const isAppend = mode === "append";
      const cursor = isAppend
        ? (activityNextCursorRef.current ?? undefined)
        : undefined;
      if (
        isAppend &&
        (!cursor ||
          isActivityLoadingMoreRef.current ||
          !hasMoreActivitiesRef.current)
      ) {
        return;
      }
      if (!isAppend && isActivityLoadingRef.current) {
        return;
      }

      if (isAppend) {
        isActivityLoadingMoreRef.current = true;
        setIsActivityLoadingMore(true);
      } else {
        isActivityLoadingRef.current = true;
        setIsActivityLoading(true);
      }

      try {
        const res = await fetch(
          API.tokenActivity(address, ACTIVITY_PAGE_SIZE, cursor),
        );
        if (res.ok) {
          const data = (await res.json()) as TokenActivityPage;
          const incoming = data.activity || [];
          const snapshotLogId = data.snapshot_log_id ?? 0;

          startTransition(() => {
            if (mode === "append") {
              setActivities((current) =>
                mergeActivities(current, incoming, "append"),
              );
              setActivityNextCursor(data.next_cursor ?? null);
              setHasMoreActivities(Boolean(data.has_more));
              return;
            }

            if (mode === "refresh_head") {
              setActivities((current) =>
                mergeActivities(current, incoming, "prepend"),
              );
              setHasMoreActivities(
                (current) => current || Boolean(data.has_more),
              );
              setActivityNextCursor(
                (current) => current ?? data.next_cursor ?? null,
              );
              return;
            }

            setActivities(incoming);
            setActivityNextCursor(data.next_cursor ?? null);
            setHasMoreActivities(Boolean(data.has_more));
          });

          if (mode === "reset") {
            activitySnapshotLogIdRef.current = snapshotLogId;
            setTopicCursor(`token:${address}`, snapshotLogId);

            const buffered = bufferedBootstrapEnvelopesRef.current;
            bufferedBootstrapEnvelopesRef.current = [];
            activityBootstrapPendingRef.current = false;
            for (const envelope of [...buffered]
              .filter((item) => (item.log_id ?? 0) > snapshotLogId)
              .sort(
                (left, right) => (left.log_id ?? 0) - (right.log_id ?? 0),
              )) {
              applyRealtimeEnvelopeRef.current(envelope);
            }
          }

          if (mode !== "append") {
            if (
              pendingRealtimeActivitiesRef.current.length === 0 &&
              !needsActivityHeadRefreshRef.current
            ) {
              setPendingActivityRefresh(false);
            }
          }
        } else if (mode === "reset") {
          activityBootstrapPendingRef.current = false;
        }
      } catch (e) {
        console.error("Failed to fetch token activity:", e);
        if (mode === "reset") {
          activityBootstrapPendingRef.current = false;
          const buffered = bufferedBootstrapEnvelopesRef.current;
          bufferedBootstrapEnvelopesRef.current = [];
          for (const envelope of buffered) {
            applyRealtimeEnvelopeRef.current(envelope);
          }
        }
      } finally {
        if (isAppend) {
          isActivityLoadingMoreRef.current = false;
          setIsActivityLoadingMore(false);
        } else {
          isActivityLoadingRef.current = false;
          setIsActivityLoading(false);
        }
      }
    },
    [address, setTopicCursor],
  );

  const flushPendingRealtimeActivities = useCallback(() => {
    const pending = pendingRealtimeActivitiesRef.current;
    if (pending.length === 0) {
      if (!needsActivityHeadRefreshRef.current) {
        setPendingActivityRefresh(false);
      }
      return;
    }

    pendingRealtimeActivitiesRef.current = [];
    startTransition(() => {
      setActivities((current) => mergeActivities(current, pending, "prepend"));
    });

    if (!needsActivityHeadRefreshRef.current) {
      setPendingActivityRefresh(false);
    }
  }, []);

  const enqueueRealtimeActivity = useCallback((activity: TokenActivity) => {
    if (activityIsAtTopRef.current) {
      startTransition(() => {
        setActivities((current) =>
          mergeActivities(current, [activity], "prepend"),
        );
      });
      if (
        pendingRealtimeActivitiesRef.current.length === 0 &&
        !needsActivityHeadRefreshRef.current
      ) {
        setPendingActivityRefresh(false);
      }
      return;
    }

    pendingRealtimeActivitiesRef.current = mergeActivities(
      pendingRealtimeActivitiesRef.current,
      [activity],
      "prepend",
    );
    setPendingActivityRefresh(true);
  }, []);

  const refreshFromApi = useCallback(async () => {
    await Promise.all([fetchTokenDetail(), reload()]);
    if (activityIsAtTopRef.current) {
      needsActivityHeadRefreshRef.current = false;
      await fetchTokenActivityPage("refresh_head");
      return;
    }
    needsActivityHeadRefreshRef.current = true;
    setPendingActivityRefresh(true);
  }, [fetchTokenDetail, fetchTokenActivityPage, reload]);

  const scheduleRefresh = useCallback(() => {
    const now = Date.now();
    const elapsed = now - lastRefreshAtRef.current;

    if (elapsed >= WS_REFRESH_COOLDOWN_MS) {
      lastRefreshAtRef.current = now;
      void refreshFromApi();
      return;
    }

    if (refreshTimerRef.current !== null) return;
    refreshTimerRef.current = window.setTimeout(() => {
      refreshTimerRef.current = null;
      lastRefreshAtRef.current = Date.now();
      void refreshFromApi();
    }, WS_REFRESH_COOLDOWN_MS - elapsed);
  }, [refreshFromApi]);

  useEffect(() => {
    if (!isConnected) {
      wasConnectedRef.current = false;
      return;
    }

    wasConnectedRef.current = true;

    if (!hasConnectedOnceRef.current) {
      hasConnectedOnceRef.current = true;
    }
  }, [isConnected]);

  const applyRealtimeEnvelope = useCallback(
    (envelope: TokenEventEnvelope) => {
      if (!eventBelongsToMint(envelope, address)) {
        return;
      }

      if (typeof envelope.log_id === "number" && envelope.log_id > 0) {
        activitySnapshotLogIdRef.current = Math.max(
          activitySnapshotLogIdRef.current,
          envelope.log_id,
        );
        setTopicCursor(`token:${address}`, envelope.log_id);
      }

      const detailSnapshot = tokenDetailRef.current;
      const nextActivity = buildRealtimeActivity(detailSnapshot, envelope);
      if (nextActivity) {
        enqueueRealtimeActivity(nextActivity);
      }

      const candlePatch = buildRealtimeCandlePatch(detailSnapshot, envelope);
      if (candlePatch) {
        const nextCandles = applyTradePatchToCandles(
          candlesRef.current,
          candlePatch,
          resolutionRef.current,
        );

        if (nextCandles.needsReload) {
          setLiveCandle(undefined);
          scheduleRefresh();
        } else {
          const latestPatchedCandle =
            nextCandles.candles[nextCandles.candles.length - 1];
          realtimeCandlePatchRef.current = true;
          candlesRef.current = nextCandles.candles;
          setCandles(nextCandles.candles);
          setLiveCandle(latestPatchedCandle);
        }
      }

      if (!nextActivity && !candlePatch) {
        scheduleRefresh();
      }

      setTokenDetail((current) => {
        if (!current || current.mint !== address) {
          return current;
        }

        const existingEvents = current.recent_events || [];
        const nextEvents = [
          envelope,
          ...existingEvents.filter(
            (item) => item.event_id !== envelope.event_id,
          ),
        ].slice(0, 40);

        return {
          ...current,
          recent_events: nextEvents,
          market_metrics: candlePatch
            ? {
                ...current.market_metrics,
                latest_price: candlePatch.price,
                latest_event_unix_ts: candlePatch.eventUnixTs,
              }
            : current.market_metrics,
        };
      });
    },
    [
      address,
      enqueueRealtimeActivity,
      scheduleRefresh,
      setCandles,
      setTopicCursor,
    ],
  );

  useEffect(() => {
    applyRealtimeEnvelopeRef.current = applyRealtimeEnvelope;
  }, [applyRealtimeEnvelope]);

  useEffect(() => {
    const removeListener = addMessageListener((raw) => {
      const msg = raw as WsMessage;
      const eventType = msg.event_type?.toLowerCase();
      if (!eventType) return;

      if (eventType === "token_stat" || eventType === "token-stat") {
        const stat = msg.payload as TokenStat | undefined;
        if (!stat) return;
        if (stat.mint && stat.mint !== address) return;
        setLiveStat(stat);
        return;
      }

      const envelope = msg as unknown as TokenEventEnvelope;
      if (eventBelongsToMint(envelope, address)) {
        if (activityBootstrapPendingRef.current) {
          bufferedBootstrapEnvelopesRef.current = mergeBufferedEnvelopes(
            bufferedBootstrapEnvelopesRef.current,
            [envelope],
          );
          return;
        }
        applyRealtimeEnvelope(envelope);
      }
    });

    return () => {
      if (refreshTimerRef.current !== null) {
        window.clearTimeout(refreshTimerRef.current);
        refreshTimerRef.current = null;
      }
      removeListener();
    };
  }, [address, addMessageListener, applyRealtimeEnvelope]);

  // Fetch token detail
  useEffect(() => {
    if (!address) return;
    queueMicrotask(() => {
      void Promise.all([fetchTokenDetail(), fetchTokenActivityPage("reset")]);
    });

    // Polling disabled for now
    // const interval = setInterval(fetchData, 10000);
    // return () => clearInterval(interval);
  }, [address, fetchTokenDetail, fetchTokenActivityPage]);

  useEffect(() => {
    const container = activityScrollRef.current;
    if (!container) return;

    const observer = new ResizeObserver((entries) => {
      const entry = entries[0];
      if (!entry) return;
      setActivityViewportHeight(entry.contentRect.height);
    });

    observer.observe(container);
    setActivityViewportHeight(container.clientHeight);

    return () => observer.disconnect();
  }, [address]);

  const handleActivityScroll = useCallback(() => {
    const container = activityScrollRef.current;
    if (!container) return;

    const nextScrollTop = container.scrollTop;
    const isAtTop = nextScrollTop <= ACTIVITY_TOP_THRESHOLD;
    const distanceToBottom =
      container.scrollHeight - nextScrollTop - container.clientHeight;

    activityIsAtTopRef.current = isAtTop;

    if (activityScrollRafRef.current != null) {
      window.cancelAnimationFrame(activityScrollRafRef.current);
    }

    activityScrollRafRef.current = window.requestAnimationFrame(() => {
      activityScrollRafRef.current = null;
      setActivityScrollTop(nextScrollTop);

      if (isAtTop) {
        flushPendingRealtimeActivities();
      }

      if (
        distanceToBottom <= ACTIVITY_LOAD_MORE_THRESHOLD &&
        hasMoreActivities &&
        !isActivityLoadingMore
      ) {
        void fetchTokenActivityPage("append");
      }

      if (
        isAtTop &&
        needsActivityHeadRefreshRef.current &&
        !isActivityLoading
      ) {
        needsActivityHeadRefreshRef.current = false;
        void fetchTokenActivityPage("refresh_head");
      }
    });
  }, [
    flushPendingRealtimeActivities,
    fetchTokenActivityPage,
    hasMoreActivities,
    isActivityLoading,
    isActivityLoadingMore,
  ]);

  useEffect(() => {
    return () => {
      if (activityScrollRafRef.current != null) {
        window.cancelAnimationFrame(activityScrollRafRef.current);
      }
    };
  }, []);

  const parseAmount = (value?: string | null) => {
    if (!value) return null;
    const num = parseFloat(value);
    if (!Number.isFinite(num)) return null;
    return num;
  };

  const isBuySide = (side?: string | null) => {
    const normalized = (side || "").toLowerCase();
    return (
      normalized === "buy" ||
      normalized.includes("buy_exact_sol_in") ||
      normalized.includes("buy_exact")
    );
  };

  const isSellSide = (side?: string | null) => {
    const normalized = (side || "").toLowerCase();
    return normalized === "sell" || normalized.includes("sell_exact");
  };

  const displayActivityType = (activity: TokenActivity) => {
    const activityType = (activity.activity_type || "").toLowerCase();

    if (activityType === "trade") {
      if (isBuySide(activity.side)) return "买入";
      if (isSellSide(activity.side)) return "卖出";
      return "交易";
    }
    if (activityType.includes("create_pool")) return "建池子";
    if (activityType.includes("add_liquidity")) return "加池子";
    if (activityType.includes("remove_liquidity")) return "减池子";
    if (activityType.includes("migrate")) return "迁移";
    if (activityType.includes("create")) return "创建";
    return activity.activity_type || activity.event_type || "--";
  };

  const copyToClipboard = useCallback((text: string, field: string) => {
    navigator.clipboard.writeText(text);
    setCopiedField(field);
    setTimeout(() => setCopiedField(null), 2000);
  }, []);

  const computePriceChange = (startPrice: number, currentPrice: number) => {
    if (!startPrice || !currentPrice) return null;
    return ((currentPrice - startPrice) / startPrice) * 100;
  };

  const currentLiveStat = liveStat?.mint === address ? liveStat : null;

  const getActiveStats = () => {
    if (currentLiveStat) {
      return {
        priceChange: computePriceChange(
          currentLiveStat.p24h,
          currentLiveStat.p,
        ),
        buys: currentLiveStat.b24h || 0,
        sells: currentLiveStat.s24h || 0,
        buyVolume: currentLiveStat.bv24h || 0,
        sellVolume: currentLiveStat.sv24h || 0,
        volume: currentLiveStat.v24h || 0,
        txns: (currentLiveStat.b24h || 0) + (currentLiveStat.s24h || 0),
        latestPrice: currentLiveStat.p,
      };
    }
    const stats24h = tokenDetail?.stats_24h;
    return {
      priceChange: tokenDetail?.price_changes?.h24 ?? null,
      buys: stats24h?.buys || 0,
      sells: stats24h?.sells || 0,
      buyVolume: stats24h?.buy_volume || 0,
      sellVolume: stats24h?.sell_volume || 0,
      volume: stats24h?.volume || 0,
      txns: stats24h?.txns,
      latestPrice: tokenDetail?.market_metrics?.latest_price,
    };
  };

  const activeStats = getActiveStats();
  const latestPrice = activeStats.latestPrice;
  const quoteSymbol = resolveQuoteSymbol(tokenDetail);
  const windowStats = {
    "1m": currentLiveStat
      ? {
          priceChange: computePriceChange(
            currentLiveStat.p1m,
            currentLiveStat.p,
          ),
          volume: currentLiveStat.v1m,
          buys: currentLiveStat.b1m,
          sells: currentLiveStat.s1m,
          buyVolume: currentLiveStat.bv1m,
          sellVolume: currentLiveStat.sv1m,
        }
      : {
          priceChange: null,
          volume: null,
          buys: null,
          sells: null,
          buyVolume: null,
          sellVolume: null,
        },
    "5m": currentLiveStat
      ? {
          priceChange: computePriceChange(
            currentLiveStat.p5m,
            currentLiveStat.p,
          ),
          volume: currentLiveStat.v5m,
          buys: currentLiveStat.b5m,
          sells: currentLiveStat.s5m,
          buyVolume: currentLiveStat.bv5m,
          sellVolume: currentLiveStat.sv5m,
        }
      : {
          priceChange: tokenDetail?.price_changes?.m5 ?? null,
          volume: null,
          buys: null,
          sells: null,
          buyVolume: null,
          sellVolume: null,
        },
    "1h": currentLiveStat
      ? {
          priceChange: computePriceChange(
            currentLiveStat.p1h,
            currentLiveStat.p,
          ),
          volume: currentLiveStat.v1h,
          buys: currentLiveStat.b1h,
          sells: currentLiveStat.s1h,
          buyVolume: currentLiveStat.bv1h,
          sellVolume: currentLiveStat.sv1h,
        }
      : {
          priceChange: tokenDetail?.price_changes?.h1 ?? null,
          volume: null,
          buys: null,
          sells: null,
          buyVolume: null,
          sellVolume: null,
        },
    "4h": currentLiveStat
      ? {
          priceChange: computePriceChange(
            currentLiveStat.p4h,
            currentLiveStat.p,
          ),
          volume: currentLiveStat.v4h,
          buys: currentLiveStat.b4h,
          sells: currentLiveStat.s4h,
          buyVolume: currentLiveStat.bv4h,
          sellVolume: currentLiveStat.sv4h,
        }
      : {
          priceChange: tokenDetail?.price_changes?.h4 ?? null,
          volume: null,
          buys: null,
          sells: null,
          buyVolume: null,
          sellVolume: null,
        },
    "24h": currentLiveStat
      ? {
          priceChange: computePriceChange(
            currentLiveStat.p24h,
            currentLiveStat.p,
          ),
          volume: currentLiveStat.v24h,
          buys: currentLiveStat.b24h,
          sells: currentLiveStat.s24h,
          buyVolume: currentLiveStat.bv24h,
          sellVolume: currentLiveStat.sv24h,
        }
      : {
          priceChange: tokenDetail?.price_changes?.h24 ?? null,
          volume: tokenDetail?.stats_24h?.volume ?? null,
          buys: tokenDetail?.stats_24h?.buys ?? null,
          sells: tokenDetail?.stats_24h?.sells ?? null,
          buyVolume: tokenDetail?.stats_24h?.buy_volume ?? null,
          sellVolume: tokenDetail?.stats_24h?.sell_volume ?? null,
        },
  } satisfies Record<
    StatsWindow,
    {
      priceChange: number | null;
      volume: number | null;
      buys: number | null;
      sells: number | null;
      buyVolume: number | null;
      sellVolume: number | null;
    }
  >;
  const currentWindowStats = windowStats[selectedStatsWindow];
  const windowBuys = currentWindowStats.buys;
  const windowSells = currentWindowStats.sells;
  const windowBuyVolume = currentWindowStats.buyVolume;
  const windowSellVolume = currentWindowStats.sellVolume;
  const netBuyVolume =
    windowBuyVolume == null || windowSellVolume == null
      ? null
      : windowBuyVolume - windowSellVolume;
  const marketCapQuote = computeMarketCapQuote(tokenDetail, latestPrice);
  const liquidityQuote = computeLiquidityQuote(tokenDetail, latestPrice);
  const migrateMarkers = useMemo(
    () => buildMigrateMarkers(tokenDetail, resolution),
    [resolution, tokenDetail],
  );
  const priceChangeWindows: Array<{
    label: StatsWindow;
    value: number | null;
  }> = [
    { label: "1m", value: windowStats["1m"].priceChange },
    { label: "5m", value: windowStats["5m"].priceChange },
    { label: "1h", value: windowStats["1h"].priceChange },
    { label: "4h", value: windowStats["4h"].priceChange },
    { label: "24h", value: windowStats["24h"].priceChange },
  ];
  const compactMarketStats = [
    {
      label: "🌊 成交额",
      value: formatCompactValue(currentWindowStats.volume),
      tone: "neutral" as MetricTone,
    },
    {
      label: "🟢 买入",
      value: formatCountAndValue(windowBuys, windowBuyVolume),
      tone: "positive" as MetricTone,
    },
    {
      label: "🔴 卖出",
      value: formatCountAndValue(windowSells, windowSellVolume),
      tone: "negative" as MetricTone,
    },
    {
      label: "⚖️ 净买入",
      value: formatCompactValue(netBuyVolume, true),
      tone: metricTone(netBuyVolume),
    },
  ];
  const rawProtocolBadge =
    tokenDetail?.create_event?.protocol || tokenDetail?.active_market?.protocol;
  const protocolBadge =
    rawProtocolBadge === "pumpamm"
      ? "PumpSwap"
      : rawProtocolBadge === "pumpfun"
        ? "Pumpfun"
        : humanizeLabel(rawProtocolBadge);
  const createdAtTs = tokenDetail?.create_event?.event_unix_ts ?? null;
  const createdAtDisplay =
    createdAtTs != null
      ? new Date(
          createdAtTs > 1e11 ? createdAtTs : createdAtTs * 1000,
        ).toLocaleString("zh-CN")
      : "--";
  const activityVisibleCount = Math.max(
    1,
    Math.ceil(activityViewportHeight / TRADE_ROW_HEIGHT),
  );
  const activityVisibleStart = Math.max(
    0,
    Math.floor(activityScrollTop / TRADE_ROW_HEIGHT) -
      ACTIVITY_VIRTUAL_OVERSCAN,
  );
  const activityVisibleEnd = Math.min(
    activities.length,
    activityVisibleStart + activityVisibleCount + ACTIVITY_VIRTUAL_OVERSCAN * 2,
  );
  const visibleActivities = useMemo(
    () =>
      activities
        .slice(activityVisibleStart, activityVisibleEnd)
        .map((item, idx) => ({
          item,
          index: activityVisibleStart + idx,
        })),
    [activities, activityVisibleEnd, activityVisibleStart],
  );
  const activityCanvasHeight = activities.length * TRADE_ROW_HEIGHT;

  return (
    <main className="grid grid-cols-1 lg:grid-cols-4">
      {/* Left Col: Chart & Live Trades */}
      <div className="col-span-1 lg:col-span-3 flex flex-col lg:border-r border-(--border-primary) order-1">
        {/* Chart */}
        <div className="h-125 border-b border-(--border-primary) shrink-0">
          <TradingChart
            data={candles}
            liveTick={liveCandle}
            markers={migrateMarkers}
            initialTimeframe={
              resolution as "1m" | "5m" | "15m" | "1h" | "4h" | "1d"
            }
            onTimeframeChange={(tf) => setResolution(tf)}
            onLoadMoreHistory={loadMoreHistory}
            canLoadMoreHistory={isHistoryLoaded && hasMoreHistory}
            isLoadingMoreHistory={isLoadingMoreHistory}
          />
        </div>

        {/* Activity */}
        <div
          className="flex flex-col bg-(--bg-secondary) overflow-hidden"
          style={{ height: "calc(100vh - 56px)" }}
        >
          <div className="px-4 py-3 border-b border-(--border-primary) bg-(--bg-secondary) flex items-center justify-between">
            <div className="flex items-center gap-2">
              <span className="live-dot" />
              <h3 className="font-semibold text-(--text-primary) text-sm">
                🧾 活动
              </h3>
            </div>
          </div>

          <div className="table-header grid grid-cols-7 gap-0 px-4 py-2">
            <div>时间</div>
            <div>类型</div>
            <div>价格</div>
            <div>数量</div>
            <div>SOL</div>
            <div>用户</div>
            <div className="text-right">Tx</div>
          </div>

          <div
            ref={activityScrollRef}
            onScroll={handleActivityScroll}
            className="flex-1 overflow-auto scrollbar-thin"
          >
            {activities.length === 0 && !isActivityLoading ? (
              <div className="flex items-center justify-center p-8 text-(--text-disabled) text-sm">
                等待活动...
              </div>
            ) : (
              <div
                className="relative"
                style={{
                  height: Math.max(
                    activityCanvasHeight + TRADE_ROW_HEIGHT,
                    activityViewportHeight,
                  ),
                }}
              >
                {visibleActivities.map(({ item: activity, index }) => {
                  const activityType = (
                    activity.activity_type || ""
                  ).toLowerCase();
                  const isBuy =
                    activityType === "trade" && isBuySide(activity.side);
                  const isSell =
                    activityType === "trade" && isSellSide(activity.side);
                  const isAddLiq = activityType.includes("add_liquidity");
                  const isRemLiq = activityType.includes("remove_liquidity");
                  const isCreate = activityType.includes("create");
                  const isMigrate = activityType.includes("migrate");

                  let priceDisplay =
                    activity.price != null ? formatPrice(activity.price) : "--";

                  const quantity = parseAmount(activity.quantity);
                  const totalQuote = parseAmount(activity.total_quote);

                  let tokenAmt =
                    quantity != null ? formatAmount(quantity) : "--";
                  let quoteAmt =
                    totalQuote != null ? totalQuote.toFixed(3) : "--";

                  if (isAddLiq || isCreate) {
                    priceDisplay = "--";
                    tokenAmt =
                      quantity != null ? `+ ${formatAmount(quantity)}` : "--";
                    quoteAmt =
                      totalQuote != null ? `+ ${totalQuote.toFixed(3)}` : "--";
                  } else if (isRemLiq) {
                    priceDisplay = "--";
                    tokenAmt =
                      quantity != null ? `- ${formatAmount(quantity)}` : "--";
                    quoteAmt =
                      totalQuote != null ? `- ${totalQuote.toFixed(3)}` : "--";
                  } else if (isMigrate) {
                    priceDisplay = "--";
                  }

                  const rowColorClass =
                    isBuy || isAddLiq
                      ? "text-(--accent-green)"
                      : isSell || isRemLiq
                        ? "text-(--accent-red)"
                        : "text-(--text-primary)";

                  const badgeClass = isBuy
                    ? "badge-buy"
                    : isSell
                      ? "badge-sell"
                      : "bg-(--bg-tertiary) text-(--text-primary)";

                  return (
                    <div
                      key={activity.event_id}
                      className={clsx(
                        "absolute left-0 right-0 w-full grid grid-cols-7 gap-0 px-4 items-center text-[13px] border-b border-(--border-primary)/50 transition-colors duration-200 hover:bg-(--bg-tertiary)",
                      )}
                      style={{
                        height: TRADE_ROW_HEIGHT,
                        top: index * TRADE_ROW_HEIGHT,
                      }}
                    >
                      <div
                        className="text-(--text-muted) font-mono text-xs"
                        title={new Date(
                          activity.event_unix_ts > 1e11
                            ? activity.event_unix_ts
                            : activity.event_unix_ts * 1000,
                        ).toLocaleString()}
                      >
                        <RelativeAgeText unixTs={activity.event_unix_ts} />
                      </div>
                      <div>
                        <span className={clsx("badge", badgeClass)}>
                          {displayActivityType(activity)}
                        </span>
                      </div>
                      <div className={clsx("font-mono", rowColorClass)}>
                        {priceDisplay}
                      </div>
                      <div className={clsx("font-mono", rowColorClass)}>
                        {tokenAmt}
                      </div>
                      <div
                        className={clsx("font-mono font-medium", rowColorClass)}
                      >
                        {quoteAmt}
                      </div>
                      <div className="font-mono text-(--text-muted) text-xs">
                        {activity.user_address
                          ? formatAddress(activity.user_address)
                          : "--"}
                      </div>
                      <div className="text-right">
                        <a
                          href={`https://solscan.io/tx/${activity.tx_signature}`}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="inline-flex items-center justify-center w-6 h-6 rounded hover:bg-(--bg-tertiary) text-(--text-muted) hover:text-(--accent-green) transition-colors"
                        >
                          <ExternalLink size={12} />
                        </a>
                      </div>
                    </div>
                  );
                })}

                <div
                  className="absolute left-0 right-0 flex items-center justify-center text-xs text-(--text-muted)"
                  style={{
                    top: activityCanvasHeight,
                    height: TRADE_ROW_HEIGHT,
                  }}
                >
                  {isActivityLoading || isActivityLoadingMore
                    ? "加载中..."
                    : pendingActivityRefresh
                      ? "回到顶部后刷新最新活动"
                      : hasMoreActivities
                        ? "继续下滑加载更多"
                        : activities.length > 0
                          ? "已经到底了"
                          : ""}
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Right Col: Token Info + Stats */}
      <div className="col-span-1 bg-(--bg-secondary) order-2 border-t lg:border-t-0 border-(--border-primary)">
        <div className="p-4 space-y-3">
          <div className="rounded-2xl border border-(--border-primary) bg-(--bg-tertiary)/50 p-3">
            <div className="flex items-start gap-3">
              {tokenDetail?.image_uri ? (
                <img
                  src={tokenDetail.image_uri}
                  alt={tokenDetail.symbol || "token"}
                  className="w-10 h-10 rounded-full object-cover ring-1 ring-(--border-secondary) shrink-0"
                  onError={(e) => {
                    (e.target as HTMLImageElement).style.display = "none";
                  }}
                />
              ) : (
                <div className="w-10 h-10 rounded-full bg-(--bg-elevated) ring-1 ring-(--border-secondary) shrink-0 flex items-center justify-center text-sm text-(--text-muted) font-bold">
                  {tokenDetail?.symbol?.[0] || "?"}
                </div>
              )}

              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2 min-w-0">
                  <span className="font-semibold text-(--text-primary) text-base truncate">
                    {tokenDetail?.name || formatAddress(address)}
                  </span>
                  {tokenDetail?.symbol && (
                    <span className="shrink-0 rounded-md bg-(--bg-elevated) px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-(--text-muted)">
                      {tokenDetail.symbol}
                    </span>
                  )}
                </div>

                <div className="mt-1 flex items-center gap-2 text-xs text-(--text-muted)">
                  <span className="font-mono">{formatAddress(address)}</span>
                  <button
                    onClick={() => copyToClipboard(address, "mint")}
                    className="hover:text-(--text-primary) transition-colors"
                    title="Copy token address"
                  >
                    {copiedField === "mint" ? (
                      <Check size={12} className="text-(--accent-green)" />
                    ) : (
                      <Copy size={12} />
                    )}
                  </button>
                  <a
                    href={`https://solscan.io/token/${address}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="hover:text-(--accent-green) transition-colors"
                  >
                    <ExternalLink size={12} />
                  </a>
                </div>
              </div>

              <span className="shrink-0 rounded-md border border-(--accent-green)/20 bg-(--accent-green)/10 px-2 py-1 text-[10px] font-semibold uppercase tracking-[0.16em] text-(--accent-green)">
                {protocolBadge !== "--" ? protocolBadge : "Pumpfun"}
              </span>
            </div>

            <div className="mt-3 rounded-2xl border border-(--border-primary) bg-(--bg-secondary)/90 p-3">
              <div className="mt-1 text-xs text-(--text-muted)">
                以 {quoteSymbol} 计价
              </div>

              <div className="mt-3 grid grid-cols-2 gap-2">
                <MiniInfoCell
                  label="⏱ 代币时长"
                  value={
                    createdAtTs != null ? (
                      <RelativeAgeText unixTs={createdAtTs} />
                    ) : (
                      "--"
                    )
                  }
                  toneClass="text-amber-300"
                />
                <MiniInfoCell
                  label="🗓 创建于"
                  value={createdAtDisplay}
                  toneClass="text-(--text-primary)"
                />
                <MiniInfoCell
                  label="💎 市值"
                  value={formatQuoteValue(marketCapQuote)}
                  toneClass="text-sky-300"
                />
                <MiniInfoCell
                  label="🔥 流动性"
                  value={formatQuoteValue(liquidityQuote)}
                  toneClass="text-amber-300"
                />
              </div>
            </div>
          </div>

          <div className="rounded-2xl border border-(--border-primary) bg-(--bg-tertiary)/40 p-1.5">
            <div className="grid grid-cols-5 gap-1.5">
              {priceChangeWindows.map((window) => (
                <CompactMetricCell
                  key={window.label}
                  label={window.label}
                  value={formatSignedPercent(window.value)}
                  tone={metricTone(window.value)}
                  emphasis="medium"
                  align="center"
                  selected={selectedStatsWindow === window.label}
                  onClick={() => setSelectedStatsWindow(window.label)}
                />
              ))}
            </div>
          </div>

          <div className="rounded-2xl border border-(--border-primary) bg-(--bg-tertiary)/40 p-1.5">
            <div className="grid grid-cols-4 gap-1.5">
              {compactMarketStats.map((item) => (
                <CompactMetricCell
                  key={item.label}
                  label={item.label}
                  value={item.value}
                  tone={item.tone}
                  emphasis="medium"
                />
              ))}
            </div>
          </div>

          <div className="rounded-2xl border border-(--border-primary) overflow-hidden">
            <div className="px-3 py-2.5 border-b border-(--border-primary) bg-(--bg-tertiary)/35 text-[11px] font-semibold uppercase tracking-[0.18em] text-(--text-muted)">
              代币信息
            </div>
            <AddressInfoRow
              label="代币地址"
              value={tokenDetail?.mint || address}
              href={`https://solscan.io/token/${address}`}
              onCopy={() =>
                copyToClipboard(tokenDetail?.mint || address, "base-mint")
              }
              copied={copiedField === "base-mint"}
            />
            {tokenDetail?.create_event?.creator && (
              <AddressInfoRow
                label="创建者"
                value={tokenDetail.create_event.creator}
                href={`https://solscan.io/account/${tokenDetail.create_event.creator}`}
                onCopy={() =>
                  copyToClipboard(tokenDetail.create_event!.creator!, "creator")
                }
                copied={copiedField === "creator"}
              />
            )}
            {tokenDetail?.active_market?.market_id && (
              <AddressInfoRow
                label="市场地址"
                value={tokenDetail.active_market.market_id}
                href={`https://solscan.io/account/${tokenDetail.active_market.market_id}`}
              />
            )}
          </div>
        </div>
      </div>
    </main>
  );
}

function CompactMetricCell({
  label,
  value,
  tone = "neutral",
  emphasis = "medium",
  align = "left",
  selected = false,
  onClick,
}: {
  label: string;
  value: string;
  tone?: MetricTone;
  emphasis?: "medium" | "large";
  align?: "left" | "center";
  selected?: boolean;
  onClick?: () => void;
}) {
  const content = (
    <>
      <div className="text-[11px] font-medium text-(--text-muted)">{label}</div>
      <div
        className={clsx(
          "mt-1.5 leading-tight break-words",
          emphasis === "large"
            ? "text-[15px] font-semibold lg:text-[18px]"
            : "text-[12px] font-mono font-semibold lg:text-[13px]",
          tone === "positive"
            ? "text-(--accent-green)"
            : tone === "negative"
              ? "text-(--accent-red)"
              : "text-(--text-secondary)",
        )}
      >
        {value}
      </div>
    </>
  );

  const className = clsx(
    "rounded-xl px-2 py-3 transition-colors",
    align === "center" ? "text-center" : "text-left",
    selected
      ? "bg-(--bg-elevated) ring-1 ring-(--accent-green)/40"
      : "bg-(--bg-secondary)",
    onClick && "cursor-pointer hover:bg-(--bg-elevated)",
  );

  if (onClick) {
    return (
      <button type="button" onClick={onClick} className={className}>
        {content}
      </button>
    );
  }

  return <div className={className}>{content}</div>;
}

function MiniInfoCell({
  label,
  value,
  toneClass = "text-(--text-primary)",
}: {
  label: string;
  value: ReactNode;
  toneClass?: string;
}) {
  return (
    <div className="rounded-xl border border-(--border-primary) bg-(--bg-tertiary)/30 px-2.5 py-2.5">
      <div className="text-[10px] uppercase tracking-[0.16em] text-(--text-muted)">
        {label}
      </div>
      <div
        className={clsx(
          "mt-1.5 break-words text-[12px] font-semibold leading-snug",
          toneClass,
        )}
      >
        {value}
      </div>
    </div>
  );
}

function AddressInfoRow({
  label,
  value,
  href,
  onCopy,
  copied,
}: {
  label: string;
  value: string;
  href?: string;
  onCopy?: () => void;
  copied?: boolean;
}) {
  const displayValue = value.length > 20 ? formatAddress(value) : value;
  return (
    <div className="flex items-center justify-between gap-3 border-b border-(--border-primary) px-3 py-3 last:border-b-0">
      <div className="min-w-0">
        <div className="text-[11px] uppercase tracking-[0.18em] text-(--text-muted)">
          {label}
        </div>
        <span className="mt-1 block truncate text-sm font-mono text-(--accent-green)">
          {displayValue}
        </span>
      </div>
      <div className="flex items-center gap-1 shrink-0">
        {onCopy && (
          <button
            onClick={onCopy}
            className="p-1 rounded hover:bg-(--bg-elevated) text-(--text-muted) hover:text-(--text-primary) transition-colors"
            title="Copy to clipboard"
          >
            {copied ? (
              <Check size={12} className="text-(--accent-green)" />
            ) : (
              <Copy size={12} />
            )}
          </button>
        )}
        {href && (
          <a
            href={href}
            target="_blank"
            rel="noopener noreferrer"
            className="p-1 rounded hover:bg-(--bg-elevated) text-(--text-muted) hover:text-(--accent-green) transition-colors"
            title="View on Solscan"
          >
            <ExternalLink size={12} />
          </a>
        )}
      </div>
    </div>
  );
}
