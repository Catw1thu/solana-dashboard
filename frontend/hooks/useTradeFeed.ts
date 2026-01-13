import { useState, useEffect, useCallback } from "react";
import { useSocket } from "../context/SocketContext";
import { useSocketSubscription } from "./useSocketSubscription";
import { Trade } from "../types";

export const useTradeFeed = (poolAddress: string) => {
  const { socket } = useSocket();
  const [trades, setTrades] = useState<Trade[]>([]);

  // 1. Subscribe to the room
  useSocketSubscription(poolAddress ? `room:${poolAddress}` : "");

  // 2. Listen for batch events
  useEffect(() => {
    if (!socket || !poolAddress) return;

    const handleBatch = (batch: Trade[]) => {
      // Prepend new trades (newest first)
      setTrades((prev) => [...batch, ...prev].slice(0, 1000)); // Limit to 1000 items in memory
    };

    socket.on("trade:batch", handleBatch);

    return () => {
      socket.off("trade:batch", handleBatch);
    };
  }, [socket, poolAddress]);

  return { trades };
};
