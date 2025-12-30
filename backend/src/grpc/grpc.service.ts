import { Injectable, OnModuleDestroy, OnModuleInit } from '@nestjs/common';
import Client, {
  CommitmentLevel,
  SubscribeRequest,
} from '@triton-one/yellowstone-grpc';
import { ConfigService } from '@nestjs/config';
import { PumpSwapParser } from '../dex-parsers/pumpSwap';
import { PumpFunParser } from '../dex-parsers/pumpFun';
import { RedisService } from 'src/redis/redis.service';

@Injectable()
export class GrpcService implements OnModuleInit, OnModuleDestroy {
  private client: Client;
  private isRunning = false;
  private stream: any;
  private pingInterval: NodeJS.Timeout | null = null;

  private poolDecimals = new Map<string, number>();
  private trackedPools = new Set<string>();

  constructor(
    private configService: ConfigService,
    private pumpSwapParser: PumpSwapParser,
    private pumpFunParser: PumpFunParser,
    private redisService: RedisService,
  ) {}

  async onModuleInit() {
    await this.loadStateFromRedis();
    await this.connect();
  }

  async onModuleDestroy() {}

  async loadStateFromRedis() {
    console.log('🔄 Loading state from Redis...');

    const trackedPools = await this.redisService.getAllTrackedPools();
    trackedPools.forEach((p) => this.trackedPools.add(p));

    const decimalsMap = await this.redisService.getAllPoolDecimals();
    for (const [pool, decimals] of Object.entries(decimalsMap)) {
      this.poolDecimals.set(pool, decimals);
    }

    console.log(`✅ State Loaded: Monitoring ${this.trackedPools.size} pools.`);
  }

  async connect() {
    const grpc_endpoint =
      this.configService.get<string>('GRPC_ENDPOINT') ??
      'solana-yellowstone-grpc.publicnode.com:443';
    console.log(`Connecting to gRPC endpoint: ${grpc_endpoint} ...`);
    this.client = new Client(grpc_endpoint, undefined, {
      'grpc.max_receive_message_length': 16 * 1024 * 1024,
    });

    try {
      const version = await this.client.getVersion();
      console.log(`✅ gRPC Connection successful! Version: ${version}`);
      this.startStream();
    } catch (error) {
      console.error(`❌ gRPC Connection failed: ${error}`);
    }
  }

  async startStream() {
    if (this.isRunning) return;
    this.isRunning = true;

    this.stream = await this.client.subscribe();

    const request: SubscribeRequest = {
      accounts: {},
      slots: {},
      transactions: {
        pumpSwap: {
          vote: false,
          failed: false,
          signature: undefined,
          accountInclude: [
            'pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA', // pumpSwap Amm
            '6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P', // pump.fun
          ],
          accountExclude: [],
          accountRequired: [],
        },
      },
      transactionsStatus: {},
      blocks: {},
      blocksMeta: {},
      entry: {},
      accountsDataSlice: [],
      commitment: CommitmentLevel.PROCESSED,
      ping: undefined,
    };

    await new Promise<void>((resolve, reject) => {
      this.stream.write(request, (err) => {
        if (err === null || err === undefined) {
          resolve();
        } else {
          reject(err);
        }
      });
    }).catch((reason) => {
      console.error(`Failed to write request to gRPC stream: ${reason}`);
      throw reason;
    });

    console.log('🚀 Listening for PumpSwap transactions...');
    this.startPing();
    this.stream.on('data', (data) => {
      if (data.transaction) {
        this.parseTx(data.transaction);
      }
    });

    this.stream.on('error', (error) => {
      console.error(`gRPC stream error: ${error}`);
      this.isRunning = false;
      this.stopPing();
    });

    this.stream.on('end', () => {
      console.log('gRPC stream ended.');
      this.isRunning = false;
      this.stopPing();
    });
  }

  private startPing() {
    if (this.pingInterval) clearInterval(this.pingInterval);

    this.pingInterval = setInterval(() => {
      if (this.stream) {
        const pingRequest: SubscribeRequest = {
          accounts: {},
          slots: {},
          transactions: {},
          transactionsStatus: {},
          blocks: {},
          blocksMeta: {},
          entry: {},
          accountsDataSlice: [],
          ping: { id: 1 },
        };
        this.stream.write(pingRequest);
      }
    }, 5000);
  }

  private stopPing() {
    if (this.pingInterval) {
      clearInterval(this.pingInterval);
      this.pingInterval = null;
    }
  }

  private parseTx(tx: any) {
    const migrateEvent = this.pumpFunParser.parseTx(tx);
    if (migrateEvent) {
      if (!this.trackedPools.has(migrateEvent.pool)) {
        //更新内存
        this.trackedPools.add(migrateEvent.pool);
        this.poolDecimals.set(migrateEvent.pool, 6);
        //异步写入redis
        this.redisService.addTrackedPool(migrateEvent.pool);
        this.redisService.setPoolDecimals(migrateEvent.pool, 6);
      }
      console.log(`\n🎓 [PUMP GRADUATION] PUMP毕业迁移!`);
      console.log(`Slot: ${migrateEvent.slot}`);
      console.log(`Tx: https://solscan.io/tx/${migrateEvent.signature}`);
      console.log(`Token Mint: ${migrateEvent.mint}`);
      console.log(`Pool: ${migrateEvent.pool}`);
      console.log(`Sol Amount: ${migrateEvent.solAmount}`);
      console.log(`Mint Amount: ${migrateEvent.tokenAmount}`);
      console.log(`Timestamp: ${migrateEvent.timestamp}`);
      return;
    }

    const event = this.pumpSwapParser.parseTx(tx);
    if (event) {
      if (event.type === 'CREATE_POOL') {
        // if (!this.trackedPools.has(event.pool)) {
        //   this.trackedPools.add(event.pool);
        //   this.poolDecimals.set(event.pool, event.baseDecimals);
        //   this.redisService.addTrackedPool(event.pool);
        //   this.redisService.setPoolDecimals(event.pool, event.baseDecimals);
        // }
        // console.log(`\n🎉 [PUMP CREATE] 新池子诞生!`);
        // console.log(`Slot: ${event.slot}`);
        // console.log(`Tx: https://solscan.io/tx/${event.signature}`);
        // console.log(`Pool: ${event.pool}`);
        // console.log(`User: ${event.creator}`);
        // console.log(`Token: ${event.baseMint}`);
        // console.log(`Init Liquidity: ${event.quoteAmount}`);
        // console.log(`Timestamp: ${event.timestamp}`);
        // console.log(`BaseDecimals: ${event.baseDecimals}`);
        // console.log(`QuoteDecimals: ${event.quoteDecimals}`);
      }
      if (event.type === 'BUY' && this.trackedPools.has(event.pool)) {
        console.log(`\n🟢 [PUMP BUY] 发生买入交易!`);
        console.log(`Slot: ${event.slot}`);
        console.log(`Tx: https://solscan.io/tx/${event.signature}`);
        console.log(`Pool: ${event.pool}`);
        console.log(`User: ${event.user}`);
        console.log(`SOL Amount Spent: ${event.tokenAmount}`);
        console.log(`Token Amount Bought: ${event.solAmount}`);
        console.log(`Timestamp: ${event.timestamp}`);
        if (this.trackedPools.has(event.pool)) {
          const baseDecimals = this.poolDecimals.get(event.pool) ?? 6;
          const quoteDecimals = 9;

          const normalizedTokenAmount =
            Number(event.tokenAmount) / 10 ** baseDecimals;
          const normalizedSolAmount =
            Number(event.solAmount) / 10 ** quoteDecimals;
          const price = normalizedTokenAmount / normalizedSolAmount;
          console.log(
            `Normalized Token Amount Bought: ${normalizedTokenAmount}`,
          );
          console.log(`Normalized SOL Amount Spent: ${normalizedSolAmount}`);
          console.log(`Price: ${price} Token per Sol`);
        }
      }
      if (event.type === 'SELL' && this.trackedPools.has(event.pool)) {
        console.log(`\n🔴 [PUMP SELL] 发生卖出交易!`);
        console.log(`Slot: ${event.slot}`);
        console.log(`Tx: https://solscan.io/tx/${event.signature}`);
        console.log(`Pool: ${event.pool}`);
        console.log(`User: ${event.user}`);
        console.log(`SOL Amount Received: ${event.tokenAmount}`);
        console.log(`Token Amount Sold: ${event.solAmount}`);
        console.log(`Timestamp: ${event.timestamp}`);
        if (this.trackedPools.has(event.pool)) {
          const baseDecimals = this.poolDecimals.get(event.pool) ?? 6;
          const quoteDecimals = 9;

          const normalizedTokenAmount =
            Number(event.tokenAmount) / 10 ** baseDecimals;
          const normalizedSolAmount =
            Number(event.solAmount) / 10 ** quoteDecimals;
          const price = normalizedTokenAmount / normalizedSolAmount;
          console.log(
            `Normalized Token Amount Bought: ${normalizedTokenAmount}`,
          );
          console.log(`Normalized SOL Amount Spent: ${normalizedSolAmount}`);
          console.log(`Price: ${price} Token per Sol`);
        }
      }
    }
  }
}
