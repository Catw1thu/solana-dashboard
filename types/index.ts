export interface TokenTradeStats {
  txns: number;
  buys: number;
  sells: number;
  volume: number;
  buy_volume: number;
  sell_volume: number;
  makers: number;
  buyers: number;
  sellers: number;
}

export interface TokenListItem {
  mint: string;
  name?: string;
  symbol?: string;
  uri?: string;
  image_uri?: string;
  creator?: string;
  bonding_curve?: string;
  token_program?: string;
  decimals?: number;
  quote_mint?: string;
  quote_decimals?: number;
  accepted_at: number;
  active_since: number;
  current_stage: string;
  current_market_type?: string;
  current_market_id?: string;
  migrated_at?: number;
  latest_price?: number;
  latest_event_unix_ts?: number;
  price_change?: number;
  window_volume: number;
  window_txns: number;
  window_buys: number;
  window_sells: number;
  liquidity_quote?: number;
  market_cap_quote?: number;
  stats_24h?: TokenTradeStats;
}

export interface TokenCreateSummary {
  event_id: string;
  protocol: string;
  event_type: string;
  event_unix_ts: number;
  creator?: string;
  bonding_curve?: string;
  name: string;
  symbol: string;
  uri: string;
  image_uri?: string;
  token_total_supply?: string;
  token_program?: string;
  is_mayhem_mode?: boolean;
  decimals?: number;
}

export interface TokenMarketRecord {
  market_id: string;
  mint: string;
  protocol: string;
  market_type: string;
  bonding_curve?: string;
  pool?: string;
  base_mint?: string;
  quote_mint?: string;
  base_mint_decimals?: number;
  quote_mint_decimals?: number;
  lp_mint?: string;
  started_at: number;
  ended_at?: number;
  create_event_id: string;
}

export interface TokenQuote {
  mint: string;
  decimals?: number;
  symbol: string;
}

export interface TokenMarketMetrics {
  latest_price?: number;
  latest_event_unix_ts?: number;
}

export interface TokenPriceChanges {
  m5?: number;
  h1?: number;
  h4?: number;
  h6?: number;
  h24?: number;
}

export interface TokenTrade {
  event_id: string;
  mint: string;
  market_id: string;
  market_type: string;
  protocol: string;
  side: string;
  ix_name: string;
  user_address: string;
  quote_mint: string;
  token_amount_raw: string;
  quote_amount_raw: string;
  token_amount: string;
  quote_amount: string;
  price?: number;
  tx_signature: string;
  slot: number;
  event_unix_ts: number;
  raw_event_source: string;
}

export interface TokenDetail {
  mint: string;
  name?: string;
  symbol?: string;
  uri?: string;
  image_uri?: string;
  creator?: string;
  bonding_curve?: string;
  token_program?: string;
  decimals?: number;
  total_supply_raw?: string;
  current_stage: string;
  create_event?: TokenCreateSummary;
  active_market?: TokenMarketRecord;
  markets: TokenMarketRecord[];
  recent_trades: TokenTrade[];
  recent_events?: TokenEventEnvelope[];
  migrate_event?: TokenEventEnvelope;
  quote?: TokenQuote;
  market_metrics?: TokenMarketMetrics;
  price_changes?: TokenPriceChanges;
  stats_24h?: TokenTradeStats;
}

export interface TokenEventRefs {
  mint?: string;
  pool?: string;
  bonding_curve?: string;
  user?: string;
  creator?: string;
  base_mint?: string;
  quote_mint?: string;
  lp_mint?: string;
}

export interface TokenEventEnvelope {
  schema_version?: number;
  event_id: string;
  chain?: string;
  protocol: string;
  event_type: string;
  commitment?: string;
  slot?: number;
  tx_signature?: string;
  tx_index?: number;
  event_source?: string;
  event_unix_ts: number;
  refs?: TokenEventRefs;
  payload?: Record<string, unknown>;
}

export interface TokenCandle {
  time: number;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
  is_gapfill?: boolean;
}

export interface TokenActivity {
  event_id: string;
  mint: string;
  protocol: string;
  event_type: string;
  activity_type: string;
  market_id?: string;
  market_type?: string;
  user_address?: string;
  side?: string;
  price?: number;
  quantity?: string;
  total_quote?: string;
  quote_mint?: string;
  tx_signature: string;
  slot: number;
  event_unix_ts: number;
  details: Record<string, unknown>;
}

export interface TokenActivityCursor {
  event_unix_ts: number;
  slot: number;
  insert_seq: number;
}

export interface TokenActivityPage {
  mint: string;
  count: number;
  has_more: boolean;
  next_cursor?: TokenActivityCursor;
  activity: TokenActivity[];
}

// Real-time stats payload from StatsBroadcaster
export interface TokenStat {
  mint: string;
  p: number;   // current price
  t: number;   // timestamp

  p1m: number;  b1m: number;  s1m: number;  bv1m: number;  sv1m: number;  v1m: number;
  p5m: number;  b5m: number;  s5m: number;  bv5m: number;  sv5m: number;  v5m: number;
  p1h: number;  b1h: number;  s1h: number;  bv1h: number;  sv1h: number;  v1h: number;
  p4h: number;  b4h: number;  s4h: number;  bv4h: number;  sv4h: number;  v4h: number;
  p24h: number; b24h: number; s24h: number; bv24h: number; sv24h: number; v24h: number;
}
