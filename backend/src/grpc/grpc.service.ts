import { Injectable, OnModuleInit } from '@nestjs/common';
import Client, {
  CommitmentLevel,
  SubscribeRequest,
} from '@triton-one/yellowstone-grpc';
import { ConfigService } from '@nestjs/config';
import { PumpSwapParser } from '../dex-parsers/pumpSwap';
import { PumpFunParser } from '../dex-parsers/pumpFun';

@Injectable()
export class GrpcService implements OnModuleInit {
  private client: Client;
  private isRunning = false;
  private poolDecimals = new Map<string, number>();
  private trackedPools = new Set<string>();

  constructor(
    private configService: ConfigService,
    private pumpSwapParser: PumpSwapParser,
    private pumpFunParser: PumpFunParser,
  ) {}

  async onModuleInit() {
    await this.connect();
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

    const stream = await this.client.subscribe();

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
      stream.write(request, (err) => {
        if (err === null || err === undefined) {
          resolve();
        } else {
          reject(err);
        }
      });
    }).catch((reason) => {
      console.error(
        `Failed to write raydium request to gRPC stream: ${reason}`,
      );
      throw reason;
    });

    console.log('🚀 Listening for PumpSwap transactions...');
    stream.on('data', (data) => {
      if (data.transaction) {
        this.parseTx(data.transaction);
      }
    });

    stream.on('error', (error) => {
      console.error(`gRPC stream error: ${error}`);
      this.isRunning = false;
    });

    stream.on('end', () => {
      console.log('gRPC stream ended.');
      this.isRunning = false;
    });
  }

  private parseTx(tx: any) {
    const migrateEvent = this.pumpFunParser.parseTx(tx);
    if (migrateEvent) {
      this.trackedPools.add(migrateEvent.pool);
      console.log(`\n🎉 [PUMP GRADUATION] PUMP迁移!`);
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
        this.trackedPools.add(event.pool);
        this.poolDecimals.set(event.pool, event.baseDecimals);
        console.log(`\n🎉 [PUMP GRADUATION] 新池子诞生!`);
        console.log(`Slot: ${event.slot}`);
        console.log(`Tx: https://solscan.io/tx/${event.signature}`);
        console.log(`Pool: ${event.pool}`);
        console.log(`User: ${event.creator}`);
        console.log(`Token: ${event.baseMint}`);
        console.log(`Init Liquidity: ${event.quoteAmount}`);
        console.log(`Timestamp: ${event.timestamp}`);
        console.log(`BaseDecimals: ${event.baseDecimals}`);
        console.log(`QuoteDecimals: ${event.quoteDecimals}`);
      }
      if (event.type === 'BUY' && this.trackedPools.has(event.pool)) {
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
      if (event.type === 'SELL' && this.trackedPools.has(event.pool)) {
        console.log(`\n🟢 [PUMP BUY] 发生买入交易!`);
        console.log(`Slot: ${event.slot}`);
        console.log(`Tx: https://solscan.io/tx/${event.signature}`);
        console.log(`Pool: ${event.pool}`);
        console.log(`User: ${event.user}`);
        console.log(`SOL Amount Spent: ${event.tokenAmount}`);
        console.log(`Token Amount Bought: ${event.solAmount}`);
        console.log(`Price: ${event.price}`);
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
