import { Injectable } from '@nestjs/common';
import { DatabaseService } from 'src/database/database.service';

export interface Candle {
  time: Date;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
}

@Injectable()
export class TokenService {
  constructor(private db: DatabaseService) {}

  /**
   * 获取单个池子详情
   */
  async getPool(address: string) {
    return this.db.pool.findUnique({
      where: { address },
    });
  }

  /**
   * 获取最新毕业的池子列表 (不带统计)
   */
  async getPools(limit = 20) {
    return this.db.pool.findMany({
      take: limit,
      orderBy: {
        createdAt: 'desc',
      },
    });
  }

  /**
   * 获取池子列表，附带基础统计指标 (price, price change, volume, txns)
   * 用一条 SQL 完成 JOIN 聚合，避免 N+1 查询
   */
  async getPoolsWithStats(limit = 20, offset = 0) {
    const now = new Date();
    const t5m = new Date(now.getTime() - 5 * 60 * 1000);

    const rows = await this.db.$queryRaw<any[]>`
      SELECT
        p.address,
        p."baseMint",
        p."quoteMint",
        p."baseDecimals",
        p."quoteDecimals",
        p.name,
        p.symbol,
        p.image,
        p."createdAt",
        -- latest price: subquery for the most recent trade
        (SELECT price FROM "Trade" WHERE "poolAddress" = p.address ORDER BY time DESC LIMIT 1) AS price,
        -- price 5m ago
        (SELECT price FROM "Trade" WHERE "poolAddress" = p.address AND time <= ${t5m} ORDER BY time DESC LIMIT 1) AS price_5m_ago,
        -- 5m aggregates
        COALESCE((SELECT SUM("quoteAmount") FROM "Trade" WHERE "poolAddress" = p.address AND time >= ${t5m}), 0) AS volume_5m,
        COALESCE((SELECT COUNT(*)::int FROM "Trade" WHERE "poolAddress" = p.address AND time >= ${t5m}), 0) AS txns_5m,
        p."baseReserves",
        p."quoteReserves",
        COALESCE((SELECT COUNT(CASE WHEN type = 'BUY' THEN 1 END)::int FROM "Trade" WHERE "poolAddress" = p.address AND time >= ${t5m}), 0) AS buys_5m,
        COALESCE((SELECT COUNT(CASE WHEN type = 'SELL' THEN 1 END)::int FROM "Trade" WHERE "poolAddress" = p.address AND time >= ${t5m}), 0) AS sells_5m
      FROM "Pool" p
      ORDER BY p."createdAt" DESC
      LIMIT ${limit}
      OFFSET ${offset};
    `;

    return rows.map((r) => ({
      address: r.address,
      baseMint: r.baseMint,
      quoteMint: r.quoteMint,
      baseDecimals: r.baseDecimals,
      quoteDecimals: r.quoteDecimals,
      name: r.name,
      symbol: r.symbol,
      image: r.image,
      createdAt: r.createdAt,
      price: r.price,
      priceChange5m:
        r.price && r.price_5m_ago && r.price_5m_ago !== 0
          ? ((r.price - r.price_5m_ago) / r.price_5m_ago) * 100
          : null,
      volume5m: r.volume_5m,
      txns5m: r.txns_5m,
      buys5m: r.buys_5m,
      sells5m: r.sells_5m,
      // Liquidity = quote reserves (SOL)
      liquidity: r.quoteReserves ?? null,
      // MCap = price × 1B (PumpFun total supply)
      mcap: r.price ? r.price * 1_000_000_000 : null,
    }));
  }

  /**
   * 获取某个池子的 K 线数据 (OHLCV)
   */
  async getOHLCV(
    poolAddress: string,
    resolution: string,
    from?: Date,
    to?: Date,
  ): Promise<Candle[]> {
    const startTime = from || new Date(Date.now() - 60 * 1000);
    const endTime = to || new Date();

    const candlse = await this.db.$queryRaw<any[]>`
    SELECT 
        time_bucket(${resolution}::interval, time) AS bucket,
        FIRST(price, time) as open,
        MAX(price) as high,
        Min(price) as low,
        LAST(price, time) as close,
        SUM("baseAmount") as volume
        FROM "Trade"
        WHERE "poolAddress" = ${poolAddress}
        AND time >= ${startTime} AND time <= ${endTime}
        GROUP BY bucket
        ORDER BY bucket ASC;
    `;
    return candlse.map((c) => ({
      time: c.bucket,
      open: c.open,
      high: c.high,
      low: c.low,
      close: c.close,
      volume: c.volume,
    }));
  }

  /**
   * 获取池子统计指标 (Volume, Price Change, Trade Count, 分窗口 Buy/Sell)
   */
  async getPoolStats(poolAddress: string) {
    const now = new Date();

    const stats = await this.db.$queryRaw<any[]>`
      SELECT
        -- Volume (SOL)
        COALESCE(SUM(CASE WHEN time >= ${new Date(now.getTime() - 5 * 60 * 1000)} THEN "quoteAmount" ELSE 0 END), 0) AS volume_5m,
        COALESCE(SUM(CASE WHEN time >= ${new Date(now.getTime() - 60 * 60 * 1000)} THEN "quoteAmount" ELSE 0 END), 0) AS volume_1h,
        COALESCE(SUM(CASE WHEN time >= ${new Date(now.getTime() - 6 * 60 * 60 * 1000)} THEN "quoteAmount" ELSE 0 END), 0) AS volume_6h,
        COALESCE(SUM("quoteAmount"), 0) AS volume_24h,

        -- Trade count
        COUNT(CASE WHEN time >= ${new Date(now.getTime() - 5 * 60 * 1000)} THEN 1 END)::int AS txns_5m,
        COUNT(CASE WHEN time >= ${new Date(now.getTime() - 60 * 60 * 1000)} THEN 1 END)::int AS txns_1h,
        COUNT(CASE WHEN time >= ${new Date(now.getTime() - 6 * 60 * 60 * 1000)} THEN 1 END)::int AS txns_6h,
        COUNT(*)::int AS txns_24h,

        -- Buy/Sell per window
        COUNT(CASE WHEN type = 'BUY' AND time >= ${new Date(now.getTime() - 5 * 60 * 1000)} THEN 1 END)::int AS buys_5m,
        COUNT(CASE WHEN type = 'SELL' AND time >= ${new Date(now.getTime() - 5 * 60 * 1000)} THEN 1 END)::int AS sells_5m,
        COUNT(CASE WHEN type = 'BUY' AND time >= ${new Date(now.getTime() - 60 * 60 * 1000)} THEN 1 END)::int AS buys_1h,
        COUNT(CASE WHEN type = 'SELL' AND time >= ${new Date(now.getTime() - 60 * 60 * 1000)} THEN 1 END)::int AS sells_1h,
        COUNT(CASE WHEN type = 'BUY' AND time >= ${new Date(now.getTime() - 6 * 60 * 60 * 1000)} THEN 1 END)::int AS buys_6h,
        COUNT(CASE WHEN type = 'SELL' AND time >= ${new Date(now.getTime() - 6 * 60 * 60 * 1000)} THEN 1 END)::int AS sells_6h,
        COUNT(CASE WHEN type = 'BUY' THEN 1 END)::int AS buys_24h,
        COUNT(CASE WHEN type = 'SELL' THEN 1 END)::int AS sells_24h

      FROM "Trade"
      WHERE "poolAddress" = ${poolAddress}
        AND time >= ${new Date(now.getTime() - 24 * 60 * 60 * 1000)};
    `;

    const pricePoints = await this.db.$queryRaw<any[]>`
      SELECT
        (SELECT price FROM "Trade" WHERE "poolAddress" = ${poolAddress} ORDER BY time DESC LIMIT 1) AS price_now,
        (SELECT price FROM "Trade" WHERE "poolAddress" = ${poolAddress} AND time <= ${new Date(now.getTime() - 5 * 60 * 1000)} ORDER BY time DESC LIMIT 1) AS price_5m_ago,
        (SELECT price FROM "Trade" WHERE "poolAddress" = ${poolAddress} AND time <= ${new Date(now.getTime() - 60 * 60 * 1000)} ORDER BY time DESC LIMIT 1) AS price_1h_ago,
        (SELECT price FROM "Trade" WHERE "poolAddress" = ${poolAddress} AND time <= ${new Date(now.getTime() - 6 * 60 * 60 * 1000)} ORDER BY time DESC LIMIT 1) AS price_6h_ago,
        (SELECT price FROM "Trade" WHERE "poolAddress" = ${poolAddress} AND time <= ${new Date(now.getTime() - 24 * 60 * 60 * 1000)} ORDER BY time DESC LIMIT 1) AS price_24h_ago;
    `;

    const s = stats[0] || {};
    const p = pricePoints[0] || {};

    const calcChange = (now: number | null, then: number | null) => {
      if (!now || !then || then === 0) return null;
      return ((now - then) / then) * 100;
    };

    const tradeStats = {
      price: p.price_now,
      priceChange5m: calcChange(p.price_now, p.price_5m_ago),
      priceChange1h: calcChange(p.price_now, p.price_1h_ago),
      priceChange6h: calcChange(p.price_now, p.price_6h_ago),
      priceChange24h: calcChange(p.price_now, p.price_24h_ago),
      volume5m: s.volume_5m,
      volume1h: s.volume_1h,
      volume6h: s.volume_6h,
      volume24h: s.volume_24h,
      txns5m: s.txns_5m,
      txns1h: s.txns_1h,
      txns6h: s.txns_6h,
      txns24h: s.txns_24h,
      buys5m: s.buys_5m,
      sells5m: s.sells_5m,
      buys1h: s.buys_1h,
      sells1h: s.sells_1h,
      buys6h: s.buys_6h,
      sells6h: s.sells_6h,
      buys24h: s.buys_24h,
      sells24h: s.sells_24h,
    };

    // Fetch pool reserves for liquidity + mcap
    const pool = await this.db.pool.findUnique({
      where: { address: poolAddress },
      select: { baseReserves: true, quoteReserves: true },
    });

    return {
      ...tradeStats,
      liquidity: pool?.quoteReserves ?? null,
      mcap: tradeStats.price ? tradeStats.price * 1_000_000_000 : null,
    };
  }

  /**
   * 获取最近交易记录
   */
  async getTrades(poolAddress: string, limit = 50) {
    return this.db.trade.findMany({
      where: { poolAddress },
      orderBy: { time: 'desc' },
      take: limit,
      select: {
        time: true,
        type: true,
        price: true,
        baseAmount: true,
        quoteAmount: true,
        txHash: true,
        maker: true,
      },
    });
  }
}
