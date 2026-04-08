"use client";

import {
  CandlestickSeries,
  ColorType,
  CrosshairMode,
  HistogramSeries,
  IChartApi,
  ISeriesMarkersPluginApi,
  ISeriesApi,
  SeriesMarker,
  Time,
  createSeriesMarkers,
  createChart,
} from "lightweight-charts";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import clsx from "clsx";
import { formatPrice } from "@/utils/format";

export interface CandleData {
  time: number;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
  is_gapfill?: boolean;
}

export interface ChartMarkerData {
  time: number;
  text: string;
  tooltip: string;
  color?: string;
  position?: "aboveBar" | "belowBar" | "inBar";
  shape?: "circle" | "square" | "arrowUp" | "arrowDown";
}

export type Timeframe = "1m" | "5m" | "15m" | "1h" | "4h" | "1d";

interface TradingChartProps {
  data: CandleData[];
  liveTick?: CandleData;
  markers?: ChartMarkerData[];
  initialTimeframe?: Timeframe;
  onTimeframeChange?: (tf: Timeframe) => void;
  colors?: {
    backgroundColor?: string;
    lineColor?: string;
    textColor?: string;
    areaTopColor?: string;
    areaBottomColor?: string;
  };
}

interface LegendData {
  open: string;
  high: string;
  low: string;
  close: string;
  volume: string;
  percentChange: string;
  priceChange: string;
  color: string;
}

interface HoveredMarker {
  time: string;
  tooltip: string;
}

type LegendCandle = Pick<CandleData, "open" | "high" | "low" | "close">;
type LegendVolume = { value?: number } | number | undefined;

const TIMEFRAMES: Timeframe[] = ["1m", "5m", "15m", "1h", "4h", "1d"];

function toChartTime(unixTime: number): Time {
  const timeInSeconds =
    unixTime > 1e11 ? Math.floor(unixTime / 1000) : unixTime;
  return timeInSeconds as Time;
}

function markerTimeKey(time: Time | undefined): string | null {
  if (time == null) return null;
  if (typeof time === "number") return String(time);
  if (typeof time === "string") return time;
  if ("year" in time && "month" in time && "day" in time) {
    return `${time.year}-${time.month}-${time.day}`;
  }
  return null;
}

function volumeColor(candle: Pick<CandleData, "open" | "close">): string {
  return candle.close >= candle.open
    ? "rgba(0, 207, 157, 0.5)"
    : "rgba(255, 77, 77, 0.5)";
}

function candleColors(candle: CandleData): {
  color: string;
  borderColor: string;
  wickColor: string;
} {
  if (candle.is_gapfill) {
    return {
      color: "rgba(148, 163, 184, 0.18)",
      borderColor: "rgba(148, 163, 184, 0.45)",
      wickColor: "rgba(148, 163, 184, 0.4)",
    };
  }

  const isUp = candle.close >= candle.open;
  return isUp
    ? {
        color: "#00cf9d",
        borderColor: "#00cf9d",
        wickColor: "#00cf9d",
      }
    : {
        color: "#ff4d4d",
        borderColor: "#ff4d4d",
        wickColor: "#ff4d4d",
      };
}

function formatLegendVolume(value?: number): string {
  if (value == null) return "N/A";
  if (value >= 1_000_000_000) return `${(value / 1_000_000_000).toFixed(2)}B`;
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(2)}M`;
  if (value >= 1_000) return `${(value / 1_000).toFixed(2)}K`;
  return value.toFixed(2);
}

function extractVolumeValue(volume: LegendVolume): number | undefined {
  if (typeof volume === "number") return volume;
  return volume?.value;
}

function toChartCandle(candle: CandleData) {
  return {
    ...candle,
    time: toChartTime(candle.time),
    ...candleColors(candle),
  };
}

function toVolumePoint(candle: CandleData) {
  return {
    time: toChartTime(candle.time),
    value: candle.volume,
    color: volumeColor(candle),
  };
}

function sameCandle(a?: CandleData, b?: CandleData): boolean {
  if (!a || !b) return false;
  return (
    a.time === b.time &&
    a.open === b.open &&
    a.high === b.high &&
    a.low === b.low &&
    a.close === b.close &&
    a.volume === b.volume &&
    Boolean(a.is_gapfill) === Boolean(b.is_gapfill)
  );
}

function canSkipFullSync(
  previous: CandleData[],
  next: CandleData[],
  liveTick?: CandleData,
): boolean {
  if (!liveTick || previous.length === 0 || next.length === 0) {
    return false;
  }

  if (previous === next) {
    return false;
  }

  const lastNext = next[next.length - 1];
  if (!sameCandle(lastNext, liveTick)) {
    return false;
  }

  if (next.length === previous.length) {
    if (previous[previous.length - 1]?.time !== lastNext.time) {
      return false;
    }
    for (let index = 0; index < next.length - 1; index += 1) {
      if (!sameCandle(previous[index], next[index])) {
        return false;
      }
    }
    return true;
  }

  if (next.length === previous.length + 1) {
    for (let index = 0; index < previous.length; index += 1) {
      if (!sameCandle(previous[index], next[index])) {
        return false;
      }
    }
    return true;
  }

  return false;
}

export const TradingChart = ({
  data,
  liveTick,
  markers = [],
  initialTimeframe = "1m",
  onTimeframeChange,
  colors = {
    backgroundColor: "transparent",
    lineColor: "#2962FF",
    textColor: "#88909f",
    areaTopColor: "#2962FF",
    areaBottomColor: "rgba(41, 98, 255, 0.28)",
  },
}: TradingChartProps) => {
  const chartContainerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const candlestickSeriesRef = useRef<ISeriesApi<"Candlestick"> | null>(null);
  const volumeSeriesRef = useRef<ISeriesApi<"Histogram"> | null>(null);
  const markerPluginRef = useRef<ISeriesMarkersPluginApi<Time> | null>(null);
  const markerMapRef = useRef<Map<string, ChartMarkerData>>(new Map());
  const liveTickRef = useRef<CandleData | undefined>(liveTick);

  const [timeframe, setTimeframe] = useState<Timeframe>(initialTimeframe);
  const [legend, setLegend] = useState<LegendData | null>(null);
  const [showVolume, setShowVolume] = useState(true);
  const [hoveredMarker, setHoveredMarker] = useState<HoveredMarker | null>(
    null,
  );

  const sortedData = useMemo(() => {
    return [...data].sort((a, b) => a.time - b.time);
  }, [data]);
  const chartMarkers = useMemo(
    () =>
      markers.map((marker) => ({
        time: toChartTime(marker.time),
        position: marker.position ?? "aboveBar",
        shape: marker.shape ?? "circle",
        color: marker.color ?? "#3782d0",
        text: marker.text,
      })) satisfies SeriesMarker<Time>[],
    [markers],
  );

  const previousDataRef = useRef<CandleData[]>(sortedData);
  const dataRef = useRef<CandleData[]>(sortedData);
  useEffect(() => {
    dataRef.current = sortedData;
  }, [sortedData]);
  useEffect(() => {
    liveTickRef.current = liveTick;
  }, [liveTick]);
  useEffect(() => {
    const nextMap = new Map<string, ChartMarkerData>();
    for (const marker of markers) {
      nextMap.set(markerTimeKey(toChartTime(marker.time)) ?? "", marker);
    }
    markerMapRef.current = nextMap;
  }, [markers]);

  const updateLegend = useCallback(
    (candle?: LegendCandle, volume?: LegendVolume) => {
      if (!candle) {
        setLegend(null);
        return;
      }

      const { open, close, high, low } = candle;
      const isUp = close >= open;
      const color = isUp ? "text-[#00cf9d]" : "text-[#ff4d4d]";
      const percent = open === 0 ? 0 : ((close - open) / open) * 100;
      const sign = percent >= 0 ? "+" : "";

      setLegend({
        open: formatPrice(open),
        high: formatPrice(high),
        low: formatPrice(low),
        close: formatPrice(close),
        volume: formatLegendVolume(extractVolumeValue(volume)),
        percentChange: `${sign}${percent.toFixed(2)}%`,
        priceChange: `${sign}${formatPrice(close - open)}`,
        color,
      });
    },
    [],
  );

  const applyFullChartData = useCallback(
    (candles: CandleData[]) => {
      if (!candlestickSeriesRef.current) return;
      candlestickSeriesRef.current.setData(candles.map(toChartCandle));

      if (volumeSeriesRef.current) {
        if (showVolume) {
          volumeSeriesRef.current.setData(candles.map(toVolumePoint));
          volumeSeriesRef.current.applyOptions({ visible: true });
        } else {
          volumeSeriesRef.current.applyOptions({ visible: false });
        }
      }

      const latestCandle = candles[candles.length - 1];
      queueMicrotask(() => {
        updateLegend(latestCandle, latestCandle?.volume);
      });
    },
    [showVolume, updateLegend],
  );

  const handleTimeframeClick = useCallback(
    (tf: Timeframe) => {
      setTimeframe(tf);
      onTimeframeChange?.(tf);
    },
    [onTimeframeChange],
  );

  useEffect(() => {
    if (!chartContainerRef.current) return;

    const chart = createChart(chartContainerRef.current, {
      layout: {
        background: { type: ColorType.Solid, color: colors.backgroundColor },
        textColor: colors.textColor,
      },
      width: chartContainerRef.current.clientWidth,
      height: chartContainerRef.current.clientHeight,
      grid: {
        vertLines: { color: "rgba(255, 255, 255, 0.03)" },
        horzLines: { color: "rgba(255, 255, 255, 0.03)" },
      },
      timeScale: {
        timeVisible: true,
        borderColor: "rgba(255, 255, 255, 0.1)",
        rightOffset: 12,
        shiftVisibleRangeOnNewBar: false,
      },
      rightPriceScale: {
        borderColor: "rgba(255, 255, 255, 0.1)",
      },
      crosshair: {
        mode: CrosshairMode.Normal,
      },
    });

    chartRef.current = chart;

    const candlestickSeries = chart.addSeries(CandlestickSeries, {
      upColor: "#00cf9d",
      downColor: "#ff4d4d",
      borderVisible: false,
      wickUpColor: "#00cf9d",
      wickDownColor: "#ff4d4d",
      priceFormat: {
        type: "custom",
        formatter: formatPrice,
        minMove: 0.000000000000001,
      },
    });
    candlestickSeriesRef.current = candlestickSeries;
    markerPluginRef.current = createSeriesMarkers(candlestickSeries, []);

    const volumeSeries = chart.addSeries(HistogramSeries, {
      priceFormat: { type: "volume" },
      priceScaleId: "",
    });
    volumeSeries.priceScale().applyOptions({
      scaleMargins: {
        top: 0.8,
        bottom: 0,
      },
    });
    volumeSeriesRef.current = volumeSeries;

    queueMicrotask(() => {
      applyFullChartData(dataRef.current);
    });

    chart.subscribeCrosshairMove((param) => {
      const currentData = dataRef.current;

      if (!param.time || currentData.length === 0) {
        const latestCandle = currentData[currentData.length - 1];
        updateLegend(latestCandle, latestCandle?.volume);
        setHoveredMarker(null);
        return;
      }

      const candle = param.seriesData.get(candlestickSeries) as
        | LegendCandle
        | undefined;
      const volume = param.seriesData.get(volumeSeries) as LegendVolume;

      updateLegend(candle, volume);

      const markerKey = markerTimeKey(param.time);
      if (!markerKey) {
        setHoveredMarker(null);
        return;
      }

      const marker = markerMapRef.current.get(markerKey) ?? null;
      setHoveredMarker(
        marker
          ? {
              time: markerKey,
              tooltip: marker.tooltip,
            }
          : null,
      );
    });

    const handleResize = () => {
      if (!chartContainerRef.current) return;
      chart.applyOptions({
        width: chartContainerRef.current.clientWidth,
        height: chartContainerRef.current.clientHeight,
      });
    };

    window.addEventListener("resize", handleResize);

    return () => {
      window.removeEventListener("resize", handleResize);
      chartRef.current = null;
      candlestickSeriesRef.current = null;
      volumeSeriesRef.current = null;
      markerPluginRef.current = null;
      chart.remove();
    };
  }, [
    applyFullChartData,
    colors.backgroundColor,
    colors.textColor,
    updateLegend,
  ]);

  useEffect(() => {
    const previous = previousDataRef.current;
    const next = sortedData;
    const skipFullSync = canSkipFullSync(previous, next, liveTickRef.current);

    if (!skipFullSync) {
      queueMicrotask(() => {
        applyFullChartData(next);
      });
    }

    previousDataRef.current = next;
  }, [applyFullChartData, sortedData]);

  useEffect(() => {
    if (!liveTick || !candlestickSeriesRef.current) return;

    const tickPoint = {
      ...liveTick,
      time: toChartTime(liveTick.time),
      ...candleColors(liveTick),
    };

    candlestickSeriesRef.current.update(tickPoint);

    if (volumeSeriesRef.current && showVolume) {
      volumeSeriesRef.current.update({
        time: tickPoint.time,
        value: liveTick.volume,
        color: volumeColor(liveTick),
      });
    }

    queueMicrotask(() => {
      updateLegend(liveTick, liveTick.volume);
    });
  }, [liveTick, showVolume, updateLegend]);

  useEffect(() => {
    markerPluginRef.current?.setMarkers(chartMarkers);
  }, [chartMarkers]);

  return (
    <div className="relative w-full h-full flex flex-col">
      <div className="flex items-center justify-between p-3 border-b border-(--border-primary)">
        <div className="flex gap-1">
          {TIMEFRAMES.map((tf) => (
            <button
              key={tf}
              onClick={() => handleTimeframeClick(tf)}
              className={clsx(
                "px-2 py-1 text-xs rounded hover:bg-(--bg-elevated) transition-colors",
                timeframe === tf
                  ? "text-(--accent-green) bg-(--accent-green)/10 font-semibold"
                  : "text-(--text-muted)",
              )}
            >
              {tf}
            </button>
          ))}
        </div>

        <div className="flex gap-2">
          <button
            onClick={() => setShowVolume((value) => !value)}
            className={clsx(
              "px-2 py-1 text-xs rounded border transition-colors",
              showVolume
                ? "border-(--accent-green)/50 text-(--accent-green)"
                : "border-transparent text-(--text-muted) hover:text-(--text-primary)",
            )}
          >
            Vol
          </button>
        </div>
      </div>

      <div className="absolute top-[50px] left-4 z-10 pointer-events-none text-xs font-mono space-y-1">
        {legend ? (
          <>
            <div className="flex gap-3">
              <span className={legend.color}>O: {legend.open}</span>
              <span className={legend.color}>H: {legend.high}</span>
              <span className={legend.color}>L: {legend.low}</span>
              <span className={legend.color}>C: {legend.close}</span>
              <span className={legend.color}>{legend.priceChange}</span>
              <span className={legend.color}>({legend.percentChange})</span>
            </div>
            {showVolume && (
              <div className="text-gray-400">Vol: {legend.volume}</div>
            )}
          </>
        ) : (
          <span className="text-gray-500">...</span>
        )}
      </div>

      {hoveredMarker && (
        <div className="pointer-events-none absolute right-21 top-[54px] z-10 max-w-[320px] rounded-xl border border-(--border-primary) bg-(--bg-secondary)/95 px-3 py-2 text-[12px] text-(--text-primary) shadow-lg backdrop-blur">
          {hoveredMarker.tooltip}
        </div>
      )}

      <div ref={chartContainerRef} className="w-full flex-1 min-h-0" />
    </div>
  );
};
