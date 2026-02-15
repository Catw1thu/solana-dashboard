export interface Trade {
  txHash: string;
  type: "BUY" | "SELL";
  price: number;
  baseAmount: number;
  quoteAmount: number;
  time: number;
  maker: string;
}

export interface PoolData {
  address: string;
  mint: string;
  solAmount: string;
  tokenAmount: string;
  timestamp: number;
  name?: string;
  symbol?: string;
  image?: string;
  // Stats from poolsWithStats endpoint
  price?: number | null;
  priceChange5m?: number | null;
  volume5m?: number;
  txns5m?: number;
  buys5m?: number;
  sells5m?: number;
}

export interface PoolDetail {
  address: string;
  baseMint: string;
  quoteMint: string;
  baseDecimals: number;
  quoteDecimals: number;
  name: string | null;
  symbol: string | null;
  image: string | null;
  createdAt: string;
}

export interface PoolStats {
  price: number | null;
  priceChange5m: number | null;
  priceChange1h: number | null;
  priceChange6h: number | null;
  priceChange24h: number | null;
  volume5m: number;
  volume1h: number;
  volume6h: number;
  volume24h: number;
  txns5m: number;
  txns1h: number;
  txns6h: number;
  txns24h: number;
  buys5m: number;
  sells5m: number;
  buys1h: number;
  sells1h: number;
  buys6h: number;
  sells6h: number;
  buys24h: number;
  sells24h: number;
}
