import { Injectable, OnModuleInit, OnModuleDestroy } from '@nestjs/common';
import { EventsGateway } from './events.gateway';
import { Logger } from '@nestjs/common';

export interface TradeDisplay {
  txHash: string;
  type: 'BUY' | 'SELL';
  price: number;
  amount: number; // Base amount
  volume: number; // Quote amount (USD/SOL volume)
  time: number; // Timestamp
  maker: string;
}

@Injectable()
export class BatcherService implements OnModuleInit, OnModuleDestroy {
  private readonly BATCH_INTERVAL = 200; // ms
  // Buffer: PoolAddress -> Array of Trades
  private tradeBuffer: Map<string, TradeDisplay[]> = new Map();
  private intervalId: NodeJS.Timeout;
  private logger = new Logger('BatcherService');

  constructor(private eventsGateway: EventsGateway) {}

  onModuleInit() {
    this.startBatcher();
  }

  onModuleDestroy() {
    if (this.intervalId) {
      clearInterval(this.intervalId);
    }
  }

  /**
   * Add a trade to the buffer for a specific pool
   */
  addTrade(poolAddress: string, trade: TradeDisplay) {
    if (!this.tradeBuffer.has(poolAddress)) {
      this.tradeBuffer.set(poolAddress, []);
    }
    this.tradeBuffer[poolAddress].push(trade);
  }

  private startBatcher() {
    this.logger.log(
      `Starting BatcherService with ${this.BATCH_INTERVAL}ms interval`,
    );
    this.intervalId = setInterval(() => {
      this.flush();
    }, this.BATCH_INTERVAL);
  }

  private flush() {
    if (this.tradeBuffer.size === 0) return;

    for (const [poolAddress, trades] of this.tradeBuffer.entries()) {
      if (trades.length > 0) {
        // Emit batch event to the pool's room
        const room = `room:${poolAddress}`;

        // Payload minification could happen here if needed,
        // e.g. mapping objects to arrays to save bandwidth.
        // For now, sending the full object is safer for MVP.
        this.eventsGateway.emitToRoom(room, 'trade:batch', trades);

        // Clear the buffer for this pool
        this.tradeBuffer.set(poolAddress, []);
      }
    }
  }
}
