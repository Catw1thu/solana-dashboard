import { Injectable } from '@nestjs/common';
import bs58 from 'bs58';

export interface PumpFunMigrateEvent {
  type: 'MIGRATE';
  signature: string;
  slot: number;
  mint: string;
  pool: string;
  solAmount: bigint;
  tokenAmount: bigint;
  timestamp: number;
}

@Injectable()
export class PumpFunParser {
  // Pump.Fun Program ID
  public readonly programId = '6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P';
  // discriminator
  private readonly DISCRIMINATORS = {
    MIGRATE_IX: Buffer.from([155, 234, 231, 146, 236, 158, 162, 30]),
    MIGRATE_EVENT: Buffer.from([
      228, 69, 165, 46, 81, 203, 154, 29, 189, 233, 93, 185, 92, 148, 234, 148,
    ]),
  };

  public parseTx(tx: any): PumpFunMigrateEvent | null {
    const signature = bs58.encode(tx.transaction.signature);
    const slot = tx.slot;
    const ixs = tx.transaction.transaction.message.instructions || [];
    const accountKeys =
      tx.transaction.transaction.message.accountKeys.map((k) =>
        bs58.encode(k),
      ) || [];
    const innerIxs = tx.transaction.meta?.innerInstructions || [];

    for (let i = 0; i < ixs.length; i++) {
      const ix = ixs[i];
      const programId = accountKeys[ix.programIdIndex];
      if (programId !== this.programId) continue;

      const dataBuffer = Buffer.from(ix.data);
      if (dataBuffer.length < 8) continue;

      const discriminator = dataBuffer.subarray(0, 8);

      if (discriminator.equals(this.DISCRIMINATORS.MIGRATE_IX)) {
        if (ix.accounts.length < 24) continue;
        const mint = accountKeys[ix.accounts[2]];

        const innerIx = innerIxs.find((inner) => inner.index === i);
        if (innerIx && innerIx.instructions) {
          const eventData = this.parseInnerEvent(innerIx.instructions);
          if (eventData) {
            return {
              type: 'MIGRATE',
              signature,
              slot,
              mint,
              ...eventData,
              timestamp: Date.now(),
            };
          }
        }
      }
    }
    return null;
  }

  private parseInnerEvent(innerIxs: any[]) {
    for (const innerIx of innerIxs) {
      const data = Buffer.from(innerIx.data);

      if (data.length < 16) continue;
      const discriminator = data.subarray(0, 16);

      try {
        if (discriminator.equals(this.DISCRIMINATORS.MIGRATE_EVENT)) {
          // Layout:
          // discriminator(16) -> 0
          // user(32) -> 16
          // mint (32) -> 48
          // mint_amount (8) -> 80
          // sol_amount (8) -> 88
          // pool_migration_fee (8) -> 96
          // bonding_curve (32) -> 104
          // timestamp (8) -> 136
          // pool (32) -> 144
          const mintAmount = data.readBigUInt64LE(80);
          const solAmount = data.readBigUInt64LE(88);
          const pool = bs58.encode(data.subarray(144, 176));

          return {
            solAmount,
            tokenAmount: mintAmount,
            pool,
          };
        }
      } catch (e) {
        console.error('Failed to parse Pump.Fun migrate inner event:', e);
      }
    }
    return null;
  }
}
