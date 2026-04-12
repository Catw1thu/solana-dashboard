// API Configuration
// Uses environment variable if set, otherwise falls back to current window location

function getApiBaseUrl(): string {
  // Check for environment variable first
  if (process.env.NEXT_PUBLIC_API_URL) {
    return process.env.NEXT_PUBLIC_API_URL;
  }

  // In browser, use the current hostname with backend port
  if (typeof window !== "undefined") {
    const protocol = window.location.protocol;
    const hostname = window.location.hostname;
    return `${protocol}//${hostname}:8081`; // default Go backend port is 8081 based on env
  }

  // Server-side fallback
  return "http://localhost:8081";
}

export const API_BASE_URL = getApiBaseUrl();

function getWsBaseUrl(): string {
  if (process.env.NEXT_PUBLIC_WS_URL) {
    return process.env.NEXT_PUBLIC_WS_URL;
  }

  if (process.env.NEXT_PUBLIC_API_URL) {
    try {
      const apiUrl = new URL(process.env.NEXT_PUBLIC_API_URL);
      apiUrl.protocol = apiUrl.protocol === "https:" ? "wss:" : "ws:";
      return apiUrl.origin;
    } catch {
      // Fallback to host-based inference below.
    }
  }

  if (typeof window !== "undefined") {
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const hostname = window.location.hostname;
    return `${protocol}//${hostname}:8081`;
  }
  return "ws://localhost:8081";
}

export const WS_BASE_URL = getWsBaseUrl();
export const WS_URL = `${WS_BASE_URL}/ws`;

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
};
