import { Injectable } from '@nestjs/common';
import bs58 from 'bs58';

export interface PumpSwapCreatePoolEvent {
  type: 'CREATE_POOL';
  signature: string;
  slot: number;
  pool: string;
  creator: string;
  baseMint: string;
  quoteMint: string;
  baseAmount: bigint;
  quoteAmount: bigint;
  timestamp: number;
}

export interface PumpSwapTradeEvent {
  type: 'BUY' | 'SELL';
  signature: string;
  slot: number;
  pool: string;
  user: string;
  baseMint: string;
  quoteMint: string;
  tokenAmount: bigint;
  solAmount: bigint;
  price: number;
  timestamp: number;
}

export type PumpSwapEvent = PumpSwapCreatePoolEvent | PumpSwapTradeEvent;

@Injectable()
export class PumpSwapParser {
  // PumpSwap AMM Program ID
  public readonly programId = 'pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA';
  // discriminator
  private readonly DISCRIMINATORS = {
    BUY: Buffer.from([102, 6, 61, 18, 1, 218, 235, 234]),
    SELL: Buffer.from([51, 230, 133, 164, 1, 127, 131, 173]),
    CREATE_POOL: Buffer.from([233, 146, 209, 142, 207, 104, 64, 188]),
  };

  public parseTx(tx: any): PumpSwapEvent | null {
    const signature = bs58.encode(tx.transaction.signature);
    const slot = tx.slot;
    const ixs = tx.transaction.transaction.message.instructions || [];
    const accountKeys =
      tx.transaction.transaction.message.accountKeys.map((k) =>
        bs58.encode(k),
      ) || [];

    for (const ix of ixs) {
      const programId = accountKeys[ix.programIdIndex];
      if (programId !== this.programId) continue;

      const dataBuffer = Buffer.from(ix.data);
      if (dataBuffer.length < 8) continue;

      const discriminator = dataBuffer.subarray(0, 8);
      const restData = dataBuffer.subarray(8);

      if (discriminator.equals(this.DISCRIMINATORS.BUY)) {
        return this.parseBuyInstruction(
          restData,
          ix.accounts,
          accountKeys,
          signature,
          slot,
        );
      }
      if (discriminator.equals(this.DISCRIMINATORS.SELL)) {
        return this.parseSellInstruction(
          restData,
          ix.accounts,
          accountKeys,
          signature,
          slot,
        );
      }
      if (discriminator.equals(this.DISCRIMINATORS.CREATE_POOL)) {
        return this.parseCreatePoolInstruction(
          restData,
          ix.accounts,
          accountKeys,
          signature,
          slot,
        );
      }
    }
    return null;
  }

  private parseCreatePoolInstruction(
    data: Buffer,
    accountIndices: number[],
    allAccounts: string[],
    signature: string,
    slot: number,
  ): PumpSwapCreatePoolEvent | null {
    // Layout:
    // index: u16 (offset 0)
    // base_amount_in: u64 (offset 2)
    // quote_amount_in: u64 (offset 10)

    // Accounts:
    // pool: accounts[0]
    // creator: accounts[2]
    // base_mint: accounts[3]
    // quote_mint: accounts[4]

    if (data.length < 18 || accountIndices.length < 5) return null;

    try {
      const baseAmount = data.readBigUInt64LE(2);
      const quoteAmount = data.readBigUInt64LE(10);

      return {
        type: 'CREATE_POOL',
        signature,
        slot,
        pool: allAccounts[accountIndices[0]],
        creator: allAccounts[accountIndices[2]],
        baseMint: allAccounts[accountIndices[3]],
        quoteMint: allAccounts[accountIndices[4]],
        baseAmount,
        quoteAmount,
        timestamp: Date.now(),
      };
    } catch (e) {
      return null;
    }
  }

  private parseBuyInstruction(
    data: Buffer,
    accountIndices: number[],
    allAccounts: string[],
    signature: string,
    slot: number,
  ): PumpSwapTradeEvent | null {
    // Latout:
    // base_amount_out: u64 (offset 0)
    // max_quote_amount_in: u64 (offset 8)

    // Accounts:
    // pool: accounts[0]
    // user: accounts[1]
    // baseMint: accounts[3]
    // quoteMint: accounts[5]

    if (data.length < 16 || accountIndices.length < 5) return null;

    try {
      const baseAmountOut = data.readBigUInt64LE(0);
      const maxQuoteAmountIn = data.readBigUInt64LE(8);

      return {
        type: 'BUY',
        signature,
        slot,
        pool: allAccounts[accountIndices[0]],
        user: allAccounts[accountIndices[1]],
        baseMint: allAccounts[accountIndices[3]],
        quoteMint: allAccounts[accountIndices[4]],
        tokenAmount: baseAmountOut,
        solAmount: maxQuoteAmountIn,
        price: Number(baseAmountOut) / Number(maxQuoteAmountIn),
        timestamp: Date.now(),
      };
    } catch (e) {
      return null;
    }
  }

  private parseSellInstruction(
    data: Buffer,
    accountIndices: number[],
    allAccounts: string[],
    signature: string,
    slot: number,
  ): PumpSwapTradeEvent | null {
    // Layout:
    // base_amount_in: u64 (offset 0)
    // min_quote_amount_out: u64 (offset 8)

    // Accounts:
    // pool: accounts[0]
    // user: accounts[1]
    // baseMint: accounts[3]
    // quoteMint: accounts[4]

    if (data.length < 16 || accountIndices.length < 5) return null;

    try {
      const baseAmountIn = data.readBigUint64LE(0);
      const minQuoteAmountOut = data.readBigUint64LE(8);

      return {
        type: 'SELL',
        signature,
        slot,
        pool: allAccounts[accountIndices[0]],
        user: allAccounts[accountIndices[1]],
        baseMint: allAccounts[accountIndices[3]],
        quoteMint: allAccounts[accountIndices[4]],
        tokenAmount: baseAmountIn,
        solAmount: minQuoteAmountOut,
        price: Number(baseAmountIn) / Number(minQuoteAmountOut),
        timestamp: Date.now(),
      };
    } catch (e) {
      return null;
    }
  }
}
