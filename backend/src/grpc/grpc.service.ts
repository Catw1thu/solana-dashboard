import { Injectable, OnModuleInit } from '@nestjs/common';
import Client, {
  CommitmentLevel,
  SubscribeRequest,
} from '@triton-one/yellowstone-grpc';
import { ConfigService } from '@nestjs/config';
import { TransactionFormatter } from './transaction-formatter';
import bs58 from 'bs58';

@Injectable()
export class GrpcService implements OnModuleInit {
  private client: Client;
  private isRunning = false;
  private transactionFormatter: TransactionFormatter;

  constructor(private configService: ConfigService) {
    this.transactionFormatter = new TransactionFormatter();
  }

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
        raydium: {
          vote: false,
          failed: false,
          signature: undefined,
          accountInclude: ['675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8'],
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

    console.log('🚀 Listening for Raydium transactions...');
    stream.on('data', (data) => {
      if (data.transaction) {
        this.processTransaction(data.transaction);
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

  private processTransaction(txData: any) {
    // 提取核心数据
    const signature = bs58.encode(txData.transaction.signature);
    const slot = txData.slot;
    const logs = txData.transaction.meta?.logMessages || [];

    const rayLogRegex = /ray_log:\s*([^ ]+)/;

    for (const log of logs) {
      // 提取 Base64 字符串
      // log 格式通常是: "Program log: ray_log: <Base64_String>"
      const match = log.match(rayLogRegex);
      if (match && match[1]) {
        const base64Data = match[1];
        const swapData = this.transactionFormatter.decodeRaydiumLog(base64Data);
        if (swapData) {
          console.log(`\n-----------------------------------------`);
          console.log(`⚡️ SWAP Detected! | Slot: ${slot}`);
          console.log(`🔗 Tx: https://solscan.io/tx/${signature}`);
          console.log(`💰 Amount In:  ${swapData.amountIn}`);
          console.log(`💵 Amount Out: ${swapData.amountOut}`);
          // 注意：这里暂时还不知道是哪个币换哪个币，只知道数量
          // 下一步我们会结合 AccountKeys 来确定币种
        }
      }
    }
  }
}
