import { Injectable, OnModuleDestroy, OnModuleInit } from '@nestjs/common';
import Client, {
  CommitmentLevel,
  SubscribeRequest,
} from '@triton-one/yellowstone-grpc';
import { ConfigService } from '@nestjs/config';
import { PumpSwapParser } from '../dex-parsers/pumpSwap';
import { PumpFunParser } from '../dex-parsers/pumpFun';
import { RedisService } from 'src/redis/redis.service';
import { DatabaseService } from 'src/database/database.service';
import { MetadataService } from 'src/token/metadata.service';
import { EventsGateway } from '../events/events.gateway';
import { BatcherService } from '../events/batcher.service';

enum ConnectionState {
  DISCONNECTED,
  CONNECTING,
  CONNECTED,
  RECONNECTING,
}

@Injectable()
export class GrpcService implements OnModuleInit, OnModuleDestroy {
  private client: Client;
  private stream: any;
  private pingInterval: NodeJS.Timeout | null = null;
  private connectionState: ConnectionState = ConnectionState.DISCONNECTED;
  private readonly MAX_RETRIES = 3;
  private reconnectAttempts = 0;
  private reconnectTimeout: NodeJS.Timeout | null = null;

  private poolDecimals = new Map<string, number>();
  private trackedPools = new Set<string>();

  constructor(
    private configService: ConfigService,
    private pumpSwapParser: PumpSwapParser,
    private pumpFunParser: PumpFunParser,
    private redisService: RedisService,
    private databaseService: DatabaseService,
    private metadataService: MetadataService,
    private eventsGateway: EventsGateway,
    private batcherService: BatcherService,
  ) {}

  async onModuleInit() {
    await this.loadStateFromRedis();
    await this.connect();
  }

  async onModuleDestroy() {
    this.connectionState = ConnectionState.DISCONNECTED;
    this.clearReconnectTimeout();
    this.stopPing();
    this.closeStream();
  }

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
    if (
      this.connectionState === ConnectionState.CONNECTING ||
      this.connectionState === ConnectionState.CONNECTED
    ) {
      return;
    }
    this.connectionState = ConnectionState.CONNECTING;
    const grpc_endpoint =
      this.configService.get<string>('GRPC_ENDPOINT') ??
      'solana-yellowstone-grpc.publicnode.com:443';
    const grpc_token =
      this.configService.get<string>('GRPC_TOKEN') ?? undefined;
    console.log(`Connecting to gRPC endpoint: ${grpc_endpoint} ...`);
    this.client = new Client(grpc_endpoint, grpc_token, {
      'grpc.max_receive_message_length': 16 * 1024 * 1024,
    });

    try {
      await this.startStream();
      this.connectionState = ConnectionState.CONNECTED;
      this.reconnectAttempts = 0;
      this.startPing();
      this.bindStream();
    } catch (error) {
      this.handleDisconnect();
    }
  }

  private async startStream() {
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
  }

  private closeStream() {
    if (this.stream) {
      this.stream.removeAllListeners();
      try {
        this.stream.destroy();
      } catch (e) {
        // ignore destroy errors
      }
      this.stream = null;
    }
  }

  private bindStream() {
    this.stream.on('data', (data) => {
      if (data.transaction) {
        this.parseTx(data.transaction).catch((e) =>
          console.error(`Failed to parse transaction: ${e.message}`),
        );
      }
    });

    this.stream.on('error', (error) => {
      console.error(`gRPC stream error: ${error}`);
      this.handleDisconnect();
    });

    this.stream.on('end', () => {
      console.log('gRPC stream ended.');
      this.handleDisconnect();
    });
  }

  private handleDisconnect() {
    if (
      this.connectionState === ConnectionState.RECONNECTING ||
      this.connectionState === ConnectionState.DISCONNECTED
    ) {
      return;
    }
    this.connectionState = ConnectionState.RECONNECTING;
    this.stopPing();
    this.closeStream();
    this.reconnect();
  }

  private reconnect() {
    if (this.reconnectAttempts >= this.MAX_RETRIES) {
      console.error(`🚨 Max reconnect attempts reached. Giving up.`);
      this.connectionState = ConnectionState.DISCONNECTED;
      return;
    }
    this.reconnectAttempts++;
    this.clearReconnectTimeout();

    console.log(
      `🔄 Reconnecting in 3s (attempt ${this.reconnectAttempts}/${this.MAX_RETRIES})...`,
    );
    this.reconnectTimeout = setTimeout(async () => {
      this.connectionState = ConnectionState.DISCONNECTED;
      await this.connect();
    }, 3000);
  }

  private clearReconnectTimeout() {
    if (this.reconnectTimeout) {
      clearTimeout(this.reconnectTimeout);
      this.reconnectTimeout = null;
    }
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

  private async parseTx(tx: any) {
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

      console.log(
        [
          `🎓 [MIGRATE]`,
          `  - Tx       : https://solscan.io/tx/${migrateEvent.signature}`,
          `  - Slot     : ${migrateEvent.slot}`,
          `  - Mint     : ${migrateEvent.mint}`,
          `  - Pool     : ${migrateEvent.pool}`,
          `  - SOL      : ${migrateEvent.solAmount}`,
          `  - Token    : ${migrateEvent.tokenAmount}`,
          `  - Time     : ${new Date(migrateEvent.timestamp).toISOString()}`,
        ].join('\n'),
      );

      await this.databaseService.savePool({
        address: migrateEvent.pool,
        baseMint: migrateEvent.mint,
        quoteMint: 'So11111111111111111111111111111111111111112',
        baseDecimals: 6,
        quoteDecimals: 9,
      });

      // Use the non-blocking queue for metadata
      this.metadataService.queueFetch(migrateEvent.pool, migrateEvent.mint);

      // Emit 'pool:new' event
      this.eventsGateway.emitGlobal('pool:new', {
        address: migrateEvent.pool,
        mint: migrateEvent.mint,
        solAmount: migrateEvent.solAmount,
        tokenAmount: migrateEvent.tokenAmount,
        timestamp: migrateEvent.timestamp,
      });

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
        // console.log(
        //   [
        //     `🎉 [CREATE]`,
        //     `  - Tx       : https://solscan.io/tx/${event.signature}`,
        //     `  - Slot     : ${event.slot}`,
        //     `  - User     : ${event.creator}`,
        //     `  - Mint     : ${event.baseMint}`,
        //     `  - Pool     : ${event.pool}`,
        //     `  - Liquidity: ${event.quoteAmount}`,
        //     `  - Base/Qt  : ${event.baseDecimals} / ${event.quoteDecimals}`,
        //     `  - Time     : ${new Date(event.timestamp).toISOString()}`,
        //   ].join('\n'),
        // );
        // this.metadataService.queueFetch(event.pool, event.baseMint);
      }
      if (
        (event.type === 'BUY' || event.type === 'SELL') &&
        this.trackedPools.has(event.pool)
      ) {
        const baseDecimals = this.poolDecimals.get(event.pool) ?? 6;
        const quoteDecimals = 9;
        const baseFactor = Math.pow(10, baseDecimals);
        const quoteFactor = Math.pow(10, quoteDecimals);
        const baseAmount = Number(event.tokenAmount) / baseFactor;
        const quoteAmount = Number(event.solAmount) / quoteFactor;
        let price = 0;
        if (baseAmount > 0) {
          price = quoteAmount / baseAmount;
        }

        console.log(
          [
            `${event.type === 'BUY' ? '🟢 [BUY]' : '🔴 [SELL]'}`,
            `  - Tx       : https://solscan.io/tx/${event.signature}`,
            `  - Slot     : ${event.slot}`,
            `  - User     : ${event.user}`,
            `  - Mint     : ${event.baseMint}`,
            `  - Pool     : ${event.pool}`,
            `  - SOL      : ${quoteAmount}`,
            `  - Token    : ${baseAmount}`,
            `  - Price    : ${price}`,
            `  - Time     : ${new Date(event.timestamp).toISOString()}`,
          ].join('\n'),
        );

        await this.databaseService.saveTrade({
          txHash: event.signature,
          time: new Date(event.timestamp),
          poolAddress: event.pool,
          type: event.type,
          price,
          baseAmount,
          quoteAmount,
        });

        // Add to batcher
        this.batcherService.addTrade(event.pool, {
          txHash: `https://solscan.io/tx/${event.signature}`,
          type: event.type,
          price,
          baseAmount,
          quoteAmount,
          time: new Date(event.timestamp).getTime(),
          maker: event.user,
        });
      }
    }
  }
}
