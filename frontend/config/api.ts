// API Configuration
// Uses environment variables if set, otherwise falls back to same-origin proxy
// paths. This lets production deployments keep the backend private behind
// Caddy/Nginx while the browser only talks to /api and /ws.

function stripTrailingSlash(value: string): string {
  return value.replace(/\/+$/, "");
}

function getApiBaseUrl(): string {
  // Check for environment variable first
  if (process.env.NEXT_PUBLIC_API_URL) {
    return stripTrailingSlash(process.env.NEXT_PUBLIC_API_URL);
  }

  return "/api";
}

export const API_BASE_URL = getApiBaseUrl();

function makeWsUrlFromOrigin(origin: string): string {
  const url = new URL(origin);
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  url.pathname = "/ws";
  url.search = "";
  url.hash = "";
  return url.toString();
}

function getWsUrl(): string {
  if (process.env.NEXT_PUBLIC_WS_URL) {
    const configured = stripTrailingSlash(process.env.NEXT_PUBLIC_WS_URL);
    if (configured.startsWith("/")) {
      if (typeof window !== "undefined") {
        const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
        return `${protocol}//${window.location.host}${configured}`;
      }
      return configured;
    }
    return configured.endsWith("/ws") ? configured : `${configured}/ws`;
  }

  if (process.env.NEXT_PUBLIC_API_URL) {
    try {
      const apiUrl = new URL(
        process.env.NEXT_PUBLIC_API_URL,
        typeof window !== "undefined" ? window.location.origin : undefined,
      );
      return makeWsUrlFromOrigin(apiUrl.origin);
    } catch {
      // Fallback to host-based inference below.
    }
  }

  if (typeof window !== "undefined") {
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    return `${protocol}//${window.location.host}/ws`;
  }
  return "ws://localhost:8081/ws";
}

export const WS_URL = getWsUrl();

// Go Backend Endpoints
export const API = {
  tokenList: (
    limit: number = 25,
    view: "hot" | "new" = "hot",
    window: "1m" | "5m" | "1h" | "4h" | "24h" = "24h",
  ) => `${API_BASE_URL}/tokens?limit=${limit}&view=${view}&window=${window}`,
  searchTokens: (query: string, limit: number = 8) =>
    `${API_BASE_URL}/search/tokens?q=${encodeURIComponent(query)}&limit=${limit}`,
  tokenDetail: (mint: string) => `${API_BASE_URL}/tokens/${mint}`,
  tokenCandles: (
    mint: string,
    resolution: string,
    limit: number = 300,
    beforeTime?: number,
  ) => {
    const params = new URLSearchParams({
      resolution,
      limit: String(limit),
    });
    if (beforeTime != null) {
      params.set("before_time", String(beforeTime));
    }
    return `${API_BASE_URL}/tokens/${mint}/candles?${params.toString()}`;
  },
  tokenTrades: (mint: string, limit: number = 100) =>
    `${API_BASE_URL}/tokens/${mint}/trades?limit=${limit}`,
  tokenActivity: (
    mint: string,
    limit: number = 100,
    cursor?: {
      event_unix_ts: number;
      slot: number;
      insert_seq: number;
    },
  ) => {
    const params = new URLSearchParams({ limit: String(limit) });
    if (cursor) {
      params.set("before_time", String(cursor.event_unix_ts));
      params.set("before_slot", String(cursor.slot));
      params.set("before_seq", String(cursor.insert_seq));
    }
    return `${API_BASE_URL}/tokens/${mint}/activity?${params.toString()}`;
  },
  tokenTimeline: (mint: string, limit: number = 100) =>
    `${API_BASE_URL}/tokens/${mint}/timeline?limit=${limit}`,
  tokenEvents: (mint: string, limit: number = 100) =>
    `${API_BASE_URL}/tokens/${mint}/events?limit=${limit}`,
  opsDocker: () => `${API_BASE_URL}/ops/docker`,
};
