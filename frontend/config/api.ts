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
    return `${protocol}//${hostname}:3000`;
  }

  // Server-side fallback
  return "http://localhost:3000";
}

function getSocketUrl(): string {
  // Check for environment variable first
  if (process.env.NEXT_PUBLIC_SOCKET_URL) {
    return process.env.NEXT_PUBLIC_SOCKET_URL;
  }

  // In browser, use the current hostname with backend port
  if (typeof window !== "undefined") {
    const protocol = window.location.protocol;
    const hostname = window.location.hostname;
    return `${protocol}//${hostname}:3000/events`;
  }

  // Server-side fallback
  return "http://localhost:3000/events";
}

export const API_BASE_URL = getApiBaseUrl();
export const SOCKET_URL = getSocketUrl();

// API endpoints — all keyed by mint (baseMint), not pool address
export const API = {
  pools: `${API_BASE_URL}/api/token/pools`,
  poolsStats: `${API_BASE_URL}/api/token/pools-stats`,
  pool: (mint: string) => `${API_BASE_URL}/api/token/pool/${mint}`,
  stats: (mint: string) => `${API_BASE_URL}/api/token/stats/${mint}`,
  candles: (mint: string, resolution: string) =>
    `${API_BASE_URL}/api/token/candles/${mint}?resolution=${resolution}&from=0`,
  trades: (mint: string, limit: number = 100) =>
    `${API_BASE_URL}/api/token/trades/${mint}?limit=${limit}`,
};
