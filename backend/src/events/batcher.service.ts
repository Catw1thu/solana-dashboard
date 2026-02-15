import { Injectable, OnModuleInit, OnModuleDestroy } from '@nestjs/common';
import { EventsGateway } from './events.gateway';
import { Logger } from '@nestjs/common';

export interface TradeDisplay {
  txHash: string;
  type: 'BUY' | 'SELL';
  price: number;
  baseAmount: number; // Base amount
  quoteAmount: number; // Quote amount (USD/SOL volume)
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
   * Add a trade to the buffer for a specific mint
   */
  addTrade(mint: string, trade: TradeDisplay) {
    if (!this.tradeBuffer.has(mint)) {
      this.tradeBuffer.set(mint, []);
    }
    this.tradeBuffer.get(mint)?.push(trade);
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

    for (const [mint, trades] of this.tradeBuffer.entries()) {
      if (trades.length > 0) {
        // Emit batch event to the mint's room
        const room = `room:${mint}`;

        this.eventsGateway.emitToRoom(room, 'trade:batch', trades);

        // Clear the buffer for this mint
        this.tradeBuffer.set(mint, []);
      }
    }
  }
}
