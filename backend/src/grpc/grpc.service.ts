import { Injectable, OnModuleInit } from '@nestjs/common';
import Client, {
  CommitmentLevel,
  SubscribeRequest,
} from '@triton-one/yellowstone-grpc';
import { ConfigService } from '@nestjs/config';

@Injectable()
export class GrpcService implements OnModuleInit {
  private client: Client;
  private isRunning = false;

  constructor(private configService: ConfigService) {}

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
        const tx = data.transaction;
        const signature = Buffer.from(tx.transaction.signature).toString('hex');
        console.log(`\n[New Transaction Detected]`);
        console.log(`Slot: ${data.slot}`);
        console.log(`Raw Object Keys: ${Object.keys(tx)}`);
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
}
