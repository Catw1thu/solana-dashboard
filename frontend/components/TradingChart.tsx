"use client";

import {
  createChart,
  ColorType,
  IChartApi,
  ISeriesApi,
  Time,
  CandlestickSeries,
  HistogramSeries,
  LineSeries,
  MouseEventParams,
  SeriesMarker,
} from "lightweight-charts";
import { useEffect, useRef, useState, useMemo } from "react";
import clsx from "clsx";

export interface CandleData {
  time: number; // Unix timestamp
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
}

export type Timeframe = "1m" | "5m" | "15m" | "1h" | "4h" | "1d";

interface TradingChartProps {
  data: CandleData[];
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

// Legend Data Structure
interface LegendData {
  open: string;
  high: string;
  low: string;
  close: string;
  volume: string;
  percentChange: string;
  color: string;
}

const TIMEFRAMES: Timeframe[] = ["1m", "5m", "15m", "1h", "4h", "1d"];

export const TradingChart = ({
  data,
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

  const [timeframe, setTimeframe] = useState<Timeframe>(initialTimeframe);
  const [legend, setLegend] = useState<LegendData | null>(null);

  // Indicator Visibility State
  const [showVolume, setShowVolume] = useState(true);

  // Memoize sorted data
  const sortedData = useMemo(() => {
    return [...data].sort((a, b) => a.time - b.time);
  }, [data]);

  // Handle Timeframe Click
  const handleTimeframeClick = (tf: Timeframe) => {
    setTimeframe(tf);
    if (onTimeframeChange) onTimeframeChange(tf);
  };

  useEffect(() => {
    if (!chartContainerRef.current) return;

    // --- Chart Initialization ---
    // --- Chart Initialization ---
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
      },
      rightPriceScale: {
        borderColor: "rgba(255, 255, 255, 0.1)",
      },
      crosshair: {
        mode: 1, // CrosshairMode.Normal
      },
    });

    chartRef.current = chart;

    // --- Series ---

    // 1. Candlestick
    const candlestickSeries = chart.addSeries(CandlestickSeries, {
      upColor: "#00cf9d",
      downColor: "#ff4d4d",
      borderVisible: false,
      wickUpColor: "#00cf9d",
      wickDownColor: "#ff4d4d",
    });
    candlestickSeriesRef.current = candlestickSeries;

    // 2. Volume (Histogram)
    const volumeSeries = chart.addSeries(HistogramSeries, {
      priceFormat: { type: "volume" },
      priceScaleId: "", // Overlay on main chart, but we need to position it
    });
    // Configure volume to sit at bottom
    volumeSeries.priceScale().applyOptions({
      scaleMargins: {
        top: 0.8, // 80% empty space at top
        bottom: 0,
      },
    });
    volumeSeriesRef.current = volumeSeries;

    // --- Initial Data Population ---
    updateChartData();

    // --- Crosshair / Legend Logic ---
    chart.subscribeCrosshairMove((param) => {
      if (!param.time || !sortedData.length) {
        setLegend(null);
        return;
      }

      // Find data point
      // Note: param.seriesData returns a Map of series -> value/OHLC
      // But for custom robust logic, we can also lookup by time if we have a map,
      // or just use what lightweight charts gives us.

      const candle = param.seriesData.get(candlestickSeries) as any;
      const volume = param.seriesData.get(volumeSeries) as any;

      if (candle) {
        const open = candle.open;
        const close = candle.close;
        const high = candle.high;
        const low = candle.low;

        // Calculate color and percent
        const isUp = close >= open;
        const color = isUp ? "text-[#00cf9d]" : "text-[#ff4d4d]";
        const percent = ((close - open) / open) * 100;
        const sign = percent >= 0 ? "+" : "";

        setLegend({
          open: open.toFixed(6),
          high: high.toFixed(6),
          low: low.toFixed(6),
          close: close.toFixed(6),
          volume: volume?.value ? (volume.value / 1e9).toFixed(2) + "B" : "N/A", // Assume scaled
          // Actually raw volume might be different.
          // Let's use raw value formatting
          percentChange: `${sign}${percent.toFixed(2)}%`,
          color,
        });
      }
    });

    // Resize handler
    const handleResize = () => {
      if (chartContainerRef.current) {
        chart.applyOptions({
          width: chartContainerRef.current.clientWidth,
          height: chartContainerRef.current.clientHeight,
        });
      }
    };
    window.addEventListener("resize", handleResize);

    return () => {
      window.removeEventListener("resize", handleResize);
      chart.remove();
    };
  }, []);

  // Use another effect to update data when 'data', 'showVolume' changes
  // This separates initialization from updates
  const updateChartData = () => {
    if (!candlestickSeriesRef.current) return;

    const chartPoints = sortedData.map((d) => ({
      ...d,
      time: (d.time / 1000) as Time,
    }));

    candlestickSeriesRef.current.setData(chartPoints);

    // Volume
    if (volumeSeriesRef.current) {
      if (showVolume) {
        const volumePoints = sortedData.map((d) => ({
          time: (d.time / 1000) as Time,
          value: d.volume,
          color:
            d.close >= d.open
              ? "rgba(0, 207, 157, 0.5)"
              : "rgba(255, 77, 77, 0.5)",
        }));
        volumeSeriesRef.current.setData(volumePoints);
        volumeSeriesRef.current.applyOptions({ visible: true });
      } else {
        volumeSeriesRef.current.applyOptions({ visible: false });
      }
    }
  };

  useEffect(() => {
    updateChartData();
  }, [sortedData, showVolume]);

  return (
    <div className="relative w-full h-full flex flex-col">
      {/* Toolbar */}
      <div className="flex items-center justify-between p-3 border-b border-white/5">
        <div className="flex gap-1">
          {TIMEFRAMES.map((tf) => (
            <button
              key={tf}
              onClick={() => handleTimeframeClick(tf)}
              className={clsx(
                "px-2 py-1 text-xs rounded hover:bg-[#20242d] transition-colors",
                timeframe === tf
                  ? "text-[#00cf9d] bg-[#00cf9d]/10 font-semibold"
                  : "text-[#88909f]"
              )}
            >
              {tf}
            </button>
          ))}
        </div>

        <div className="flex gap-2">
          <button
            onClick={() => setShowVolume(!showVolume)}
            className={clsx(
              "px-2 py-1 text-xs rounded border transition-colors",
              showVolume
                ? "border-[#00cf9d]/50 text-[#00cf9d]"
                : "border-transparent text-[#88909f] hover:text-[#ffffff]"
            )}
          >
            Vol
          </button>
        </div>
      </div>

      {/* Legend */}
      <div className="absolute top-[50px] left-4 z-10 pointer-events-none text-xs font-mono space-y-1">
        {legend ? (
          <>
            <div className="flex gap-3">
              <span className={legend.color}>O: {legend.open}</span>
              <span className={legend.color}>H: {legend.high}</span>
              <span className={legend.color}>L: {legend.low}</span>
              <span className={legend.color}>C: {legend.close}</span>
              <span className={legend.color}>({legend.percentChange})</span>
            </div>
            {showVolume && (
              <div className="text-gray-400">Vol: {legend.volume}</div>
            )}
          </>
        ) : (
          // Default to showing latest data if available, or just empty
          <span className="text-gray-500">...</span>
        )}
      </div>

      <div ref={chartContainerRef} className="w-full flex-1 min-h-0" />
    </div>
  );
};
