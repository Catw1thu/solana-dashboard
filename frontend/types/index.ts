export interface Trade {
  txHash: string;
  type: "BUY" | "SELL";
  price: number;
  amount: number;
  volume: number;
  time: number;
  maker: string;
}

export interface PoolData {
  address: string;
  mint: string;
  solAmount: string;
  tokenAmount: string;
  timestamp: number;
}
