"use client";

import {
  createChart,
  ColorType,
  IChartApi,
  ISeriesApi,
  Time,
  CandlestickSeries,
} from "lightweight-charts";
import { useEffect, useRef } from "react";

export interface CandleData {
  time: number; // Unix timestamp
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
}

interface TradingChartProps {
  data: CandleData[];
  colors?: {
    backgroundColor?: string;
    lineColor?: string;
    textColor?: string;
    areaTopColor?: string;
    areaBottomColor?: string;
  };
}

export const TradingChart = ({
  data,
  colors = {
    backgroundColor: "transparent",
    lineColor: "#2962FF",
    textColor: "#A3A3A3",
    areaTopColor: "#2962FF",
    areaBottomColor: "rgba(41, 98, 255, 0.28)",
  },
}: TradingChartProps) => {
  const chartContainerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const candlestickSeriesRef = useRef<ISeriesApi<"Candlestick"> | null>(null);

  useEffect(() => {
    if (!chartContainerRef.current) return;

    const handleResize = () => {
      chartRef.current?.applyOptions({
        width: chartContainerRef.current!.clientWidth,
      });
    };

    const chart = createChart(chartContainerRef.current, {
      layout: {
        background: { type: ColorType.Solid, color: colors.backgroundColor },
        textColor: colors.textColor,
      },
      width: chartContainerRef.current.clientWidth,
      height: 400,
      grid: {
        vertLines: { color: "rgba(255, 255, 255, 0.05)" },
        horzLines: { color: "rgba(255, 255, 255, 0.05)" },
      },
      timeScale: {
        timeVisible: true,
        borderColor: "rgba(255, 255, 255, 0.1)",
      },
      rightPriceScale: {
        borderColor: "rgba(255, 255, 255, 0.1)",
      },
    });

    chartRef.current = chart;

    const candlestickSeries = chart.addSeries(CandlestickSeries, {
      upColor: "#22c55e",
      downColor: "#ef4444",
      borderVisible: false,
      wickUpColor: "#22c55e",
      wickDownColor: "#ef4444",
    });

    candlestickSeriesRef.current = candlestickSeries;

    // Sort data by time before setting
    const sortedData = [...data]
      .sort((a, b) => a.time - b.time)
      .map((item) => ({
        ...item,
        time: (item.time / 1000) as Time, // Lightweight charts expects seconds for unix timestamps
      }));

    candlestickSeries.setData(sortedData);

    window.addEventListener("resize", handleResize);

    return () => {
      window.removeEventListener("resize", handleResize);
      chart.remove();
    };
  }, []);

  // Update data when props change
  useEffect(() => {
    if (candlestickSeriesRef.current && data.length > 0) {
      const sortedData = [...data]
        .sort((a, b) => a.time - b.time)
        .map((item) => ({
          ...item,
          time: (item.time / 1000) as Time,
        }));
      candlestickSeriesRef.current.setData(sortedData);
    }
  }, [data]);

  return (
    <div className="relative w-full rounded-xl border border-white/10 bg-white/5 p-4 backdrop-blur-md">
      <div className="mb-4 flex items-center justify-between">
        <h3 className="text-lg font-semibold text-white">Price Chart</h3>
        <div className="flex gap-2">
          {/* Resolution selector could go here */}
          <span className="rounded bg-white/10 px-2 py-1 text-xs text-gray-400">
            1m
          </span>
        </div>
      </div>
      <div ref={chartContainerRef} className="w-full" />
    </div>
  );
};
