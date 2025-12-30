import { Injectable, OnModuleDestroy, OnModuleInit } from '@nestjs/common';
import Redis from 'ioredis';
import { ConfigService } from '@nestjs/config';

@Injectable()
export class RedisService implements OnModuleInit, OnModuleDestroy {
  private client: Redis;

  // key前缀,防止冲突
  private readonly KEY_TRACKED_POOLS = 'pumpswap:tracked_pools';
  private readonly KEY_POOL_DECIMALS = 'pumpswap:pool_decimals';

  constructor(private configService: ConfigService) {
    this.client = new Redis({
      host: this.configService.get('REDIS_HOST') || 'localhost',
      port: this.configService.get('REDIS_PORT')
        ? Number(this.configService.get('REDIS_PORT'))
        : 6379,
    });

    this.client.on('error', (err) => {
      console.error('❌ Redis Connection Error:', err);
    });
  }

  onModuleInit() {
    console.log('✅ Redis Connected');
  }

  onModuleDestroy() {
    this.client.disconnect();
  }

  // --- 追踪的池子管理 ---
  async addTrackedPool(poolAddress: string) {
    await this.client.sadd(this.KEY_TRACKED_POOLS, poolAddress);
  }
  async getAllTrackedPools(): Promise<string[]> {
    return await this.client.smembers(this.KEY_TRACKED_POOLS);
  }
  async isPoolTracked(poolAddress: string): Promise<boolean> {
    return (
      (await this.client.sismember(this.KEY_TRACKED_POOLS, poolAddress)) === 1
    );
  }
  // --- 池子Decimals管理 ---
  async setPoolDecimals(poolAddress: string, baseDecimals: number) {
    await this.client.hset(
      this.KEY_POOL_DECIMALS,
      poolAddress,
      baseDecimals.toString(),
    );
  }
  async getPoolDecimals(poolAddress: string): Promise<number | null> {
    const val = await this.client.hget(this.KEY_POOL_DECIMALS, poolAddress);
    return val ? parseInt(val, 10) : null;
  }
  async getAllPoolDecimals(): Promise<Record<string, number>> {
    const all = await this.client.hgetall(this.KEY_POOL_DECIMALS);
    const result: Record<string, number> = {};
    for (const [pool, decimals] of Object.entries(all)) {
      result[pool] = parseInt(decimals, 10);
    }
    return result;
  }
}
