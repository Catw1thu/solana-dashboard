import {
  TokenActivity,
  TokenCandle,
  TokenDetail,
  TokenEventEnvelope,
} from "@/types";

export type CandleResolution = "1m" | "5m" | "15m" | "1h" | "4h" | "1d";

type TradeFields = {
  mint: string;
  side: string;
  marketId?: string;
  marketType?: string;
  userAddress?: string;
  quoteMint?: string;
  tokenAmountRaw: string;
  quoteAmountRaw: string;
};

type CandlePatch = {
  eventUnixTs: number;
  price: number;
  volume: number;
};

type CandlePatchResult = {
  candles: TokenCandle[];
  needsReload: boolean;
};

const SOL_MINT = "So11111111111111111111111111111111111111112";
const DEFAULT_TOKEN_DECIMALS = 6;
const DEFAULT_QUOTE_DECIMALS = 9;
const RESOLUTION_SECONDS: Record<CandleResolution, number> = {
  "1m": 60,
  "5m": 5 * 60,
  "15m": 15 * 60,
  "1h": 60 * 60,
  "4h": 4 * 60 * 60,
  "1d": 24 * 60 * 60,
};

function readString(
  payload: Record<string, unknown> | undefined,
  key: string,
): string | undefined {
  const value = payload?.[key];
  return typeof value === "string" && value !== "" ? value : undefined;
}

function normalizeEventType(envelope: TokenEventEnvelope): string {
  return envelope.event_type.toLowerCase();
}

function isPumpfunTrade(envelope: TokenEventEnvelope): boolean {
  const eventType = normalizeEventType(envelope);
  return envelope.protocol === "pumpfun" && (eventType === "trade" || eventType === "pumpfun.trade");
}

function isPumpfunCreate(envelope: TokenEventEnvelope): boolean {
  return envelope.protocol === "pumpfun" && normalizeEventType(envelope) === "create";
}

function isPumpfunMigrate(envelope: TokenEventEnvelope): boolean {
  return envelope.protocol === "pumpfun" && normalizeEventType(envelope) === "migrate";
}

function isPumpAmmSwap(envelope: TokenEventEnvelope): boolean {
  return envelope.protocol === "pumpamm" && normalizeEventType(envelope) === "swap";
}

function isPumpAmmCreatePool(envelope: TokenEventEnvelope): boolean {
  return envelope.protocol === "pumpamm" && normalizeEventType(envelope) === "create_pool";
}

function isPumpAmmLiquidity(envelope: TokenEventEnvelope): boolean {
  if (envelope.protocol !== "pumpamm") {
    return false;
  }

  const eventType = normalizeEventType(envelope);
  return eventType === "deposit" || eventType === "withdraw";
}

function resolveNonSolMint(
  baseMint?: string,
  quoteMint?: string,
): string | undefined {
  if (baseMint === SOL_MINT && quoteMint) {
    return quoteMint;
  }
  if (quoteMint === SOL_MINT && baseMint) {
    return baseMint;
  }
  return undefined;
}

function resolveTrackedMint(
  envelope: TokenEventEnvelope,
  payload?: Record<string, unknown>,
): string | undefined {
  if (envelope.refs?.mint) {
    return envelope.refs.mint;
  }

  const payloadMint = readString(payload, "mint");
  if (payloadMint) {
    return payloadMint;
  }

  return resolveNonSolMint(
    envelope.refs?.base_mint ?? readString(payload, "base_mint"),
    envelope.refs?.quote_mint ?? readString(payload, "quote_mint"),
  );
}

function resolveTokenDecimals(
  detail: TokenDetail | null,
  mint: string,
): number {
  if (detail?.mint === mint) {
    return detail.decimals ?? detail.create_event?.decimals ?? DEFAULT_TOKEN_DECIMALS;
  }

  const activeMarket = detail?.active_market;
  if (activeMarket?.base_mint === mint) {
    return activeMarket.base_mint_decimals ?? DEFAULT_TOKEN_DECIMALS;
  }
  if (activeMarket?.quote_mint === mint) {
    return activeMarket.quote_mint_decimals ?? detail?.quote?.decimals ?? DEFAULT_TOKEN_DECIMALS;
  }

  return detail?.decimals ?? detail?.create_event?.decimals ?? DEFAULT_TOKEN_DECIMALS;
}

function resolveQuoteDecimals(
  detail: TokenDetail | null,
  quoteMint?: string,
): number {
  if (!quoteMint || quoteMint === SOL_MINT) {
    return DEFAULT_QUOTE_DECIMALS;
  }
  if (detail?.quote?.mint === quoteMint) {
    return detail.quote.decimals ?? DEFAULT_QUOTE_DECIMALS;
  }

  const activeMarket = detail?.active_market;
  if (activeMarket?.base_mint === quoteMint) {
    return activeMarket.base_mint_decimals ?? DEFAULT_QUOTE_DECIMALS;
  }
  if (activeMarket?.quote_mint === quoteMint) {
    return activeMarket.quote_mint_decimals ?? DEFAULT_QUOTE_DECIMALS;
  }

  return detail?.quote?.decimals ?? DEFAULT_QUOTE_DECIMALS;
}

function scaleRawAmount(raw: string, decimals: number): number | null {
  const numeric = Number(raw);
  if (!Number.isFinite(numeric)) {
    return null;
  }
  return numeric / 10 ** decimals;
}

function formatRawAmount(raw: string, decimals: number): string {
  try {
    const zero = BigInt(0);
    let value = BigInt(raw);
    const negative = value < zero;
    if (negative) {
      value = -value;
    }

    if (decimals <= 0) {
      return `${negative ? "-" : ""}${value.toString()}`;
    }

    let digits = value.toString();
    if (digits.length <= decimals) {
      digits = `${"0".repeat(decimals - digits.length + 1)}${digits}`;
    }

    const point = digits.length - decimals;
    let formatted = `${digits.slice(0, point)}.${digits.slice(point)}`;
    formatted = formatted.replace(/\.?0+$/, "");
    if (formatted === "") {
      formatted = "0";
    }

    return negative && formatted !== "0" ? `-${formatted}` : formatted;
  } catch {
    return raw;
  }
}

function computePrice(
  tokenAmountRaw: string,
  quoteAmountRaw: string,
  tokenDecimals: number,
  quoteDecimals: number,
): number | null {
  const tokenAmount = scaleRawAmount(tokenAmountRaw, tokenDecimals);
  const quoteAmount = scaleRawAmount(quoteAmountRaw, quoteDecimals);
  if (
    tokenAmount == null ||
    quoteAmount == null ||
    tokenAmount <= 0 ||
    !Number.isFinite(tokenAmount) ||
    !Number.isFinite(quoteAmount)
  ) {
    return null;
  }

  return quoteAmount / tokenAmount;
}

function trackedPairAmounts(
  mint: string,
  baseMint: string,
  quoteMint: string,
  baseAmount: string,
  quoteAmount: string,
): [string, string] {
  if (mint === quoteMint) {
    return [quoteAmount, baseAmount];
  }
  return [baseAmount, quoteAmount];
}

function otherMint(
  mint: string,
  baseMint: string,
  quoteMint: string,
): string | undefined {
  if (mint === baseMint) {
    return quoteMint;
  }
  if (mint === quoteMint) {
    return baseMint;
  }
  return undefined;
}

function buildActivity(
  detail: TokenDetail | null,
  envelope: TokenEventEnvelope,
  fields: {
    mint: string;
    activityType: string;
    marketId?: string;
    marketType?: string;
    userAddress?: string;
    side?: string;
    quoteMint?: string;
    tokenAmountRaw?: string;
    quoteAmountRaw?: string;
  },
): TokenActivity {
  const tokenDecimals = resolveTokenDecimals(detail, fields.mint);
  const quoteDecimals = resolveQuoteDecimals(detail, fields.quoteMint);
  const quantity = fields.tokenAmountRaw
    ? formatRawAmount(fields.tokenAmountRaw, tokenDecimals)
    : undefined;
  const totalQuote = fields.quoteAmountRaw
    ? formatRawAmount(fields.quoteAmountRaw, quoteDecimals)
    : undefined;
  const price =
    fields.tokenAmountRaw && fields.quoteAmountRaw
      ? computePrice(
          fields.tokenAmountRaw,
          fields.quoteAmountRaw,
          tokenDecimals,
          quoteDecimals,
        )
      : null;

  return {
    event_id: envelope.event_id,
    mint: fields.mint,
    protocol: envelope.protocol,
    event_type: envelope.event_type,
    activity_type: fields.activityType,
    market_id: fields.marketId,
    market_type: fields.marketType,
    user_address: fields.userAddress,
    side: fields.side,
    price: price ?? undefined,
    quantity,
    total_quote: totalQuote,
    quote_mint: fields.quoteMint,
    tx_signature: envelope.tx_signature ?? "",
    slot: envelope.slot ?? 0,
    event_unix_ts: envelope.event_unix_ts,
    details: envelope.payload ?? {},
  };
}

function buildTradeFields(
  envelope: TokenEventEnvelope,
): TradeFields | null {
  const payload = envelope.payload;
  if (!payload) {
    return null;
  }

  if (isPumpfunTrade(envelope)) {
    const mint = readString(payload, "mint") ?? envelope.refs?.mint;
    const tokenAmountRaw = readString(payload, "token_amount");
    const quoteAmountRaw = readString(payload, "sol_amount");
    if (!mint || !tokenAmountRaw || !quoteAmountRaw) {
      return null;
    }

    return {
      mint,
      side: readString(payload, "side") ?? "",
      marketId: readString(payload, "bonding_curve") ?? envelope.refs?.bonding_curve,
      marketType: "pumpfun_curve",
      userAddress: readString(payload, "user") ?? envelope.refs?.user,
      quoteMint: SOL_MINT,
      tokenAmountRaw,
      quoteAmountRaw,
    };
  }

  if (!isPumpAmmSwap(envelope)) {
    return null;
  }

  const baseMint = readString(payload, "base_mint");
  const quoteMint = readString(payload, "quote_mint");
  const mint = resolveTrackedMint(envelope, payload);
  const side = readString(payload, "side");
  if (!baseMint || !quoteMint || !mint || !side) {
    return null;
  }

  const marketId = readString(payload, "pool") ?? envelope.refs?.pool;
  const userAddress = readString(payload, "user") ?? envelope.refs?.user;
  const currentQuoteMint = otherMint(mint, baseMint, quoteMint);
  if (!currentQuoteMint) {
    return null;
  }

  if (mint === quoteMint) {
    if (side === "sell") {
      const tokenAmountRaw = readString(payload, "quote_amount_out");
      const quoteAmountRaw = readString(payload, "base_amount_in");
      if (!tokenAmountRaw || !quoteAmountRaw) {
        return null;
      }
      return {
        mint,
        side: "buy",
        marketId,
        marketType: "pumpamm_pool",
        userAddress,
        quoteMint: currentQuoteMint,
        tokenAmountRaw,
        quoteAmountRaw,
      };
    }

    if (side === "buy" || side === "buy_exact_quote_in") {
      const tokenAmountRaw = readString(payload, "quote_amount_in");
      const quoteAmountRaw = readString(payload, "base_amount_out");
      if (!tokenAmountRaw || !quoteAmountRaw) {
        return null;
      }
      return {
        mint,
        side: "sell",
        marketId,
        marketType: "pumpamm_pool",
        userAddress,
        quoteMint: currentQuoteMint,
        tokenAmountRaw,
        quoteAmountRaw,
      };
    }

    return null;
  }

  if (side === "sell") {
    const tokenAmountRaw = readString(payload, "base_amount_in");
    const quoteAmountRaw = readString(payload, "quote_amount_out");
    if (!tokenAmountRaw || !quoteAmountRaw) {
      return null;
    }
    return {
      mint,
      side: "sell",
      marketId,
      marketType: "pumpamm_pool",
      userAddress,
      quoteMint: currentQuoteMint,
      tokenAmountRaw,
      quoteAmountRaw,
    };
  }

  if (side === "buy" || side === "buy_exact_quote_in") {
    const tokenAmountRaw = readString(payload, "base_amount_out");
    const quoteAmountRaw = readString(payload, "quote_amount_in");
    if (!tokenAmountRaw || !quoteAmountRaw) {
      return null;
    }
    return {
      mint,
      side: "buy",
      marketId,
      marketType: "pumpamm_pool",
      userAddress,
      quoteMint: currentQuoteMint,
      tokenAmountRaw,
      quoteAmountRaw,
    };
  }

  return null;
}

export function eventBelongsToMint(
  envelope: TokenEventEnvelope,
  mint: string,
): boolean {
  const payloadMint = readString(envelope.payload, "mint");
  if (envelope.refs?.mint === mint || payloadMint === mint) {
    return true;
  }

  return resolveTrackedMint(envelope, envelope.payload) === mint;
}

export function buildRealtimeActivity(
  detail: TokenDetail | null,
  envelope: TokenEventEnvelope,
): TokenActivity | null {
  const payload = envelope.payload;
  if (!payload) {
    return null;
  }

  const trade = buildTradeFields(envelope);
  if (trade) {
    return buildActivity(detail, envelope, {
      mint: trade.mint,
      activityType: "trade",
      marketId: trade.marketId,
      marketType: trade.marketType,
      userAddress: trade.userAddress,
      side: trade.side,
      quoteMint: trade.quoteMint,
      tokenAmountRaw: trade.tokenAmountRaw,
      quoteAmountRaw: trade.quoteAmountRaw,
    });
  }

  if (isPumpfunCreate(envelope)) {
    const mint = readString(payload, "mint") ?? envelope.refs?.mint;
    if (!mint) {
      return null;
    }
    return buildActivity(detail, envelope, {
      mint,
      activityType: "create",
      marketId: readString(payload, "bonding_curve") ?? envelope.refs?.bonding_curve,
      marketType: "pumpfun_curve",
      userAddress: readString(payload, "user") ?? envelope.refs?.user,
    });
  }

  if (isPumpfunMigrate(envelope)) {
    const mint = readString(payload, "mint") ?? envelope.refs?.mint;
    if (!mint) {
      return null;
    }
    return buildActivity(detail, envelope, {
      mint,
      activityType: "migrate",
      marketId: readString(payload, "pool") ?? envelope.refs?.pool,
      marketType: "pumpamm_pool",
      userAddress: readString(payload, "user") ?? envelope.refs?.user,
      quoteMint: SOL_MINT,
      tokenAmountRaw: readString(payload, "mint_amount"),
      quoteAmountRaw: readString(payload, "sol_amount"),
    });
  }

  if (isPumpAmmCreatePool(envelope)) {
    const baseMint = readString(payload, "base_mint");
    const quoteMint = readString(payload, "quote_mint");
    const mint = resolveTrackedMint(envelope, payload);
    const baseAmountIn = readString(payload, "base_amount_in");
    const quoteAmountIn = readString(payload, "quote_amount_in");
    if (!baseMint || !quoteMint || !mint || !baseAmountIn || !quoteAmountIn) {
      return null;
    }

    const [tokenAmountRaw, quoteAmountRaw] = trackedPairAmounts(
      mint,
      baseMint,
      quoteMint,
      baseAmountIn,
      quoteAmountIn,
    );

    return buildActivity(detail, envelope, {
      mint,
      activityType: "create_pool",
      marketId: readString(payload, "pool") ?? envelope.refs?.pool,
      marketType: "pumpamm_pool",
      userAddress: readString(payload, "creator") ?? envelope.refs?.creator,
      quoteMint: otherMint(mint, baseMint, quoteMint),
      tokenAmountRaw,
      quoteAmountRaw,
    });
  }

  if (isPumpAmmLiquidity(envelope)) {
    const baseMint = readString(payload, "base_mint");
    const quoteMint = readString(payload, "quote_mint");
    const mint = resolveTrackedMint(envelope, payload);
    const action = readString(payload, "action") ?? normalizeEventType(envelope);
    if (!baseMint || !quoteMint || !mint || !action) {
      return null;
    }

    let tokenAmountRaw = "";
    let quoteAmountRaw = "";

    if (action === "deposit") {
      [tokenAmountRaw, quoteAmountRaw] = trackedPairAmounts(
        mint,
        baseMint,
        quoteMint,
        readString(payload, "base_amount_in") ?? "",
        readString(payload, "quote_amount_in") ?? "",
      );
    } else if (action === "withdraw") {
      [tokenAmountRaw, quoteAmountRaw] = trackedPairAmounts(
        mint,
        baseMint,
        quoteMint,
        readString(payload, "base_amount_out") ?? "",
        readString(payload, "quote_amount_out") ?? "",
      );
    }

    return buildActivity(detail, envelope, {
      mint,
      activityType: action,
      marketId: readString(payload, "pool") ?? envelope.refs?.pool,
      marketType: "pumpamm_pool",
      userAddress: readString(payload, "user") ?? envelope.refs?.user,
      quoteMint: otherMint(mint, baseMint, quoteMint),
      tokenAmountRaw: tokenAmountRaw || undefined,
      quoteAmountRaw: quoteAmountRaw || undefined,
    });
  }

  return null;
}

export function buildRealtimeCandlePatch(
  detail: TokenDetail | null,
  envelope: TokenEventEnvelope,
): CandlePatch | null {
  const trade = buildTradeFields(envelope);
  if (!trade) {
    return null;
  }

  const tokenDecimals = resolveTokenDecimals(detail, trade.mint);
  const quoteDecimals = resolveQuoteDecimals(detail, trade.quoteMint);
  const price = computePrice(
    trade.tokenAmountRaw,
    trade.quoteAmountRaw,
    tokenDecimals,
    quoteDecimals,
  );
  const volume = scaleRawAmount(trade.quoteAmountRaw, quoteDecimals);
  if (price == null || volume == null || !Number.isFinite(price) || !Number.isFinite(volume)) {
    return null;
  }

  return {
    eventUnixTs: envelope.event_unix_ts,
    price,
    volume,
  };
}

export function applyTradePatchToCandles(
  candles: TokenCandle[],
  patch: CandlePatch,
  resolution: CandleResolution,
): CandlePatchResult {
  const bucketSize = RESOLUTION_SECONDS[resolution];
  if (!bucketSize) {
    return { candles, needsReload: true };
  }

  const bucketTime = Math.floor(patch.eventUnixTs / bucketSize) * bucketSize;
  if (candles.length === 0) {
    return {
      candles: [
        {
          time: bucketTime,
          open: patch.price,
          high: patch.price,
          low: patch.price,
          close: patch.price,
          volume: patch.volume,
          is_gapfill: false,
        },
      ],
      needsReload: false,
    };
  }

  const next = candles.map((candle) => ({ ...candle }));
  const last = next[next.length - 1];
  if (bucketTime < last.time) {
    return { candles, needsReload: true };
  }

  if (bucketTime === last.time) {
    next[next.length - 1] = {
      ...last,
      high: Math.max(last.high, patch.price),
      low: Math.min(last.low, patch.price),
      close: patch.price,
      volume: last.volume + patch.volume,
      is_gapfill: false,
    };
    return { candles: next, needsReload: false };
  }

  const previousClose = last.close;
  next.push({
    time: bucketTime,
    open: previousClose,
    high: Math.max(previousClose, patch.price),
    low: Math.min(previousClose, patch.price),
    close: patch.price,
    volume: patch.volume,
    is_gapfill: false,
  });

  return { candles: next, needsReload: false };
}
