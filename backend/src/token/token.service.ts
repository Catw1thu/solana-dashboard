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
   * 获取最新毕业的池子列表
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
   * 获取某个池子的 K 线数据 (OHLCV)
   * @param poolAddress 池子地址
   * @param resolution 时间粒度 (例如 '1 minute', '1 hour', '1 day')
   * @param from 开始时间 (可选)
   * @param to 结束时间 (可选)
   */
  async getOHLCV(
    poolAddress: string,
    resolution: string,
    from?: Date,
    to?: Date,
  ): Promise<Candle[]> {
    const startTime = from || new Date(Date.now() - 60 * 1000); // 默认过去一分钟
    const endTime = to || new Date();

    // TimescaleDB 聚合查询
    // time_bucket: 按时间窗口切分
    // first/last: 获取该窗口内的开盘/收盘价 (按 time 排序)
    // max/min: 获取最高/最低价
    // sum: 计算成交量

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
      },
    });
  }
}
