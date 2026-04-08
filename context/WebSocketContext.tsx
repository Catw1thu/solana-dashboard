"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { WS_URL } from "@/config/api";

type WsMessage = {
  event_type?: string;
  payload?: unknown;
  [key: string]: unknown;
};

type MessageHandler = (msg: WsMessage) => void;

interface WebSocketContextType {
  isConnected: boolean;
  subscribe: (topic: string) => void;
  unsubscribe: (topic: string) => void;
  addMessageListener: (handler: MessageHandler) => () => void;
}

const WebSocketContext = createContext<WebSocketContextType | null>(null);

export function useAppWebSocket() {
  const ctx = useContext(WebSocketContext);
  if (!ctx) {
    throw new Error("useAppWebSocket must be used within WebSocketProvider");
  }
  return ctx;
}

export function WebSocketProvider({ children }: { children: ReactNode }) {
  const [isConnected, setIsConnected] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const connectRef = useRef<() => void>(() => {});
  const reconnectTimeoutRef = useRef<number | null>(null);
  const reconnectAttemptsRef = useRef(0);
  const manuallyClosedRef = useRef(false);

  const subscribedTopicsRef = useRef<Set<string>>(new Set());
  const topicRefCountsRef = useRef<Map<string, number>>(new Map());
  const pendingUnsubscribeRef = useRef<Map<string, number>>(new Map());
  const listenersRef = useRef<Set<MessageHandler>>(new Set());

  const reconnectBaseInterval = 1000;
  const reconnectMaxInterval = 15000;

  const sendJson = useCallback((payload: unknown) => {
    const ws = wsRef.current;
    if (!ws || ws.readyState !== WebSocket.OPEN) return false;
    ws.send(JSON.stringify(payload));
    return true;
  }, []);

  const connect = useCallback(() => {
    const existing = wsRef.current;
    if (
      existing &&
      (existing.readyState === WebSocket.OPEN ||
        existing.readyState === WebSocket.CONNECTING)
    ) {
      return;
    }

    manuallyClosedRef.current = false;
    if (reconnectTimeoutRef.current !== null) {
      window.clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }
    const ws = new WebSocket(WS_URL);
    wsRef.current = ws;

    ws.onopen = () => {
      setIsConnected(true);
      reconnectAttemptsRef.current = 0;
      console.log(`[WS] Connected to ${WS_URL}`);

      subscribedTopicsRef.current.forEach((topic) => {
        if (sendJson({ action: "subscribe", topic })) {
          console.log(`[WS] Subscribed to ${topic}`);
        }
      });
    };

    ws.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data) as WsMessage;
        listenersRef.current.forEach((handler) => handler(message));
      } catch (error) {
        console.error("[WS] Failed to parse incoming message:", error);
      }
    };

    ws.onerror = (error) => {
      console.error(
        `[WS] Connection Error to ${WS_URL}. Verify backend is active and ORIGIN matches.`,
        error,
      );
    };

    ws.onclose = (event) => {
      setIsConnected(false);
      wsRef.current = null;
      console.log(
        `[WS] Disconnected from ${WS_URL} (Code: ${event.code}, Reason: ${event.reason || "N/A"})`,
      );

      if (manuallyClosedRef.current) return;

      reconnectAttemptsRef.current += 1;
      const nextDelay = Math.min(
        reconnectBaseInterval *
          2 ** Math.max(0, reconnectAttemptsRef.current - 1),
        reconnectMaxInterval,
      );
      reconnectTimeoutRef.current = window.setTimeout(() => {
        connectRef.current();
      }, nextDelay);
    };
  }, [sendJson]);

  useEffect(() => {
    connectRef.current = connect;
  }, [connect]);

  const clearPendingUnsubscribe = useCallback((topic: string) => {
    const timeoutId = pendingUnsubscribeRef.current.get(topic);
    if (timeoutId == null) return;

    window.clearTimeout(timeoutId);
    pendingUnsubscribeRef.current.delete(topic);
  }, []);

  const disconnect = useCallback(() => {
    manuallyClosedRef.current = true;

    if (reconnectTimeoutRef.current !== null) {
      window.clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }
    pendingUnsubscribeRef.current.forEach((timeoutId) => {
      window.clearTimeout(timeoutId);
    });
    pendingUnsubscribeRef.current.clear();

    const ws = wsRef.current;
    if (!ws) return;

    if (
      ws.readyState === WebSocket.OPEN ||
      ws.readyState === WebSocket.CONNECTING
    ) {
      ws.close();
    }

    wsRef.current = null;
    setIsConnected(false);
  }, []);

  useEffect(() => {
    connectRef.current();
    return () => disconnect();
  }, [disconnect]);

  useEffect(() => {
    const reconnectIfNeeded = () => {
      const ws = wsRef.current;
      if (!ws || ws.readyState === WebSocket.CLOSED) {
        connectRef.current();
      }
    };

    const handleVisibility = () => {
      if (document.visibilityState === "visible") {
        reconnectIfNeeded();
      }
    };

    window.addEventListener("online", reconnectIfNeeded);
    document.addEventListener("visibilitychange", handleVisibility);

    return () => {
      window.removeEventListener("online", reconnectIfNeeded);
      document.removeEventListener("visibilitychange", handleVisibility);
    };
  }, []);

  const subscribe = useCallback(
    (topic: string) => {
      if (!topic) return;

      clearPendingUnsubscribe(topic);
      const currentCount = topicRefCountsRef.current.get(topic) ?? 0;
      topicRefCountsRef.current.set(topic, currentCount + 1);
      if (currentCount > 0) return;

      subscribedTopicsRef.current.add(topic);
      if (sendJson({ action: "subscribe", topic })) {
        console.log(`[WS] Subscribed to ${topic}`);
      }
    },
    [clearPendingUnsubscribe, sendJson],
  );

  const unsubscribe = useCallback(
    (topic: string) => {
      if (!topic) return;

      clearPendingUnsubscribe(topic);
      const currentCount = topicRefCountsRef.current.get(topic) ?? 0;
      if (currentCount <= 1) {
        topicRefCountsRef.current.delete(topic);
        const timeoutId = window.setTimeout(() => {
          if ((topicRefCountsRef.current.get(topic) ?? 0) > 0) {
            pendingUnsubscribeRef.current.delete(topic);
            return;
          }

          subscribedTopicsRef.current.delete(topic);
          pendingUnsubscribeRef.current.delete(topic);
          if (sendJson({ action: "unsubscribe", topic })) {
            console.log(`[WS] Unsubscribed from ${topic}`);
          }
        }, 0);

        pendingUnsubscribeRef.current.set(topic, timeoutId);
        return;
      }

      topicRefCountsRef.current.set(topic, currentCount - 1);
    },
    [clearPendingUnsubscribe, sendJson],
  );

  const addMessageListener = useCallback((handler: MessageHandler) => {
    listenersRef.current.add(handler);
    return () => {
      listenersRef.current.delete(handler);
    };
  }, []);

  const value = useMemo(
    () => ({ isConnected, subscribe, unsubscribe, addMessageListener }),
    [isConnected, subscribe, unsubscribe, addMessageListener],
  );

  return (
    <WebSocketContext.Provider value={value}>
      {children}
    </WebSocketContext.Provider>
  );
}
