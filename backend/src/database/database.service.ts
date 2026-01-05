import { Injectable, OnModuleInit } from '@nestjs/common';
import { PrismaClient } from '@prisma/client';

@Injectable()
export class DatabaseService extends PrismaClient implements OnModuleInit {
  async onModuleInit() {
    await this.$connect();
  }

  /**
   * 保存或更新池子信息
   * 场景：收到 MIGRATE 事件或 CREATE_POOL 事件时调用
   */
  async savePool(data: {
    address: string;
    baseMint: string;
    quoteMint: string;
    baseDecimals: number;
    quoteDecimals: number;
  }) {
    // upsert = update + insert
    return this.pool.upsert({
      where: { address: data.address },
      update: {},
      create: {
        address: data.address,
        baseMint: data.baseMint,
        quoteMint: data.quoteMint,
        baseDecimals: data.baseDecimals,
        quoteDecimals: data.quoteDecimals,
      },
    });
  }

  /**
   * 保存交易记录
   * 场景：收到 BUY/SELL 事件时调用
   */
  async saveTrade(data: {
    txHash: string;
    time: Date;
    poolAddress: string;
    type: 'BUY' | 'SELL';
    price: number;
    baseAmount: number;
    quoteAmount: number;
  }) {
    return this.trade.create({
      data: {
        time: data.time,
        txHash: data.txHash,
        poolAddress: data.poolAddress,
        type: data.type,
        price: data.price,
        baseAmount: data.baseAmount,
        quoteAmount: data.quoteAmount,
      },
    });
  }
}
