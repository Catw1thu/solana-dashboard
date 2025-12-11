import { Injectable } from '@nestjs/common';
import bs58 from 'bs58';
import e from 'express';

export interface PumpSwapCreatePoolEvent {
  type: 'CREATE_POOL';
  signature: string;
  slot: number;
  pool: string;
  creator: string;
  baseMint: string;
  quoteMint: string;
  baseDecimals: number;
  quoteDecimals: number;
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
  limitSolAmount: bigint;
  protocolFee: bigint;
  lpFee: bigint;
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
    // ix
    BUY_IX: Buffer.from([102, 6, 61, 18, 1, 218, 235, 234]),
    SELL_IX: Buffer.from([51, 230, 133, 164, 1, 127, 131, 173]),
    CREATE_POOL_IX: Buffer.from([233, 146, 209, 142, 207, 104, 64, 188]),
    // event
    BUY_EVENT: Buffer.from([
      228, 69, 165, 46, 81, 203, 154, 29, 103, 244, 82, 31, 44, 245, 119, 119,
    ]),
    SELL_EVENT: Buffer.from([
      228, 69, 165, 46, 81, 203, 154, 29, 62, 47, 55, 10, 165, 3, 220, 42,
    ]),
    CREATE_POOL_EVENT: Buffer.from([
      228, 69, 165, 46, 81, 203, 154, 29, 177, 49, 12, 210, 160, 118, 167, 116,
    ]),
  };

  public parseTx(tx: any): PumpSwapEvent | null {
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
      const restData = dataBuffer.subarray(8);

      let draftEvent: PumpSwapEvent | null = null;

      if (discriminator.equals(this.DISCRIMINATORS.BUY_IX)) {
        draftEvent = this.parseBuyInstruction(
          restData,
          ix.accounts,
          accountKeys,
          signature,
          slot,
        );
      }
      if (discriminator.equals(this.DISCRIMINATORS.SELL_IX)) {
        draftEvent = this.parseSellInstruction(
          restData,
          ix.accounts,
          accountKeys,
          signature,
          slot,
        );
      }
      if (discriminator.equals(this.DISCRIMINATORS.CREATE_POOL_IX)) {
        draftEvent = this.parseCreatePoolInstruction(
          restData,
          ix.accounts,
          accountKeys,
          signature,
          slot,
        );
      }

      if (draftEvent) {
        const innerIx = innerIxs.find((inner) => inner.index === i);
        if (innerIx && innerIx.instructions) {
          this.scanAndMergeInnerEvents(draftEvent, innerIx.instructions);
        }
        return draftEvent;
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
    // quote_amount_out: u64 (offset 10)

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
        baseDecimals: 6,
        quoteDecimals: 9,
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
        limitSolAmount: maxQuoteAmountIn,
        solAmount: maxQuoteAmountIn,
        protocolFee: BigInt(0),
        lpFee: BigInt(0),
        price: 0,
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
        limitSolAmount: minQuoteAmountOut,
        solAmount: minQuoteAmountOut,
        protocolFee: BigInt(0),
        lpFee: BigInt(0),
        price: 0,
        timestamp: Date.now(),
      };
    } catch (e) {
      return null;
    }
  }

  private scanAndMergeInnerEvents(
    event: PumpSwapEvent,
    innerInstructions: any[],
  ) {
    for (const innerIx of innerInstructions) {
      const data = Buffer.from(innerIx.data);
      if (data.length < 16) continue;
      const discriminator = data.subarray(0, 16);

      if (
        event.type === 'BUY' &&
        discriminator.equals(this.DISCRIMINATORS.BUY_EVENT)
      ) {
        this.mergeTradeData(event, data);
      }
      if (
        event.type === 'SELL' &&
        discriminator.equals(this.DISCRIMINATORS.SELL_EVENT)
      ) {
        this.mergeTradeData(event, data);
      }
      if (
        event.type === 'CREATE_POOL' &&
        discriminator.equals(this.DISCRIMINATORS.CREATE_POOL_EVENT)
      ) {
        this.mergeCreatePoolData(event, data);
      }
    }
  }

  private mergeTradeData(event: PumpSwapTradeEvent, data: Buffer) {
    // 0-15: Discriminator (16 bytes)
    // 16: timestamp (8)
    // 24: base_amount_out (8)
    // 32: max_quote_amount_in (8)
    // 40: user_base_token_reserves (8)
    // 48: user_quote_token_reserves (8)
    // 56: pool_base_token_reserves (8)
    // 64: pool_quote_token_reserves (8)
    // 72: quote_amount_in (8)
    // 80: lp_fee_basis_points (8)
    // 88: lp_fee (8)
    // 96: protocol_fee_basis_points (8)
    // 104: protocol_fee (8)
    // 112: quote_amount_in_with_lp_fee (8)
    // 120: user_quote_amount_in (8)
    try {
      event.solAmount = data.readBigUInt64LE(120);
      event.lpFee = data.readBigInt64LE(88);
      event.protocolFee = data.readBigInt64LE(104);

      if (Number(event.tokenAmount) > 0) {
        event.price = Number(event.tokenAmount) / Number(event.solAmount);
      }
    } catch (e) {
      console.error('Failed to merge Trade Event data', e);
    }
  }

  private mergeCreatePoolData(event: PumpSwapCreatePoolEvent, data: Buffer) {
    // Disc(16) + Timestamp(8) + Index(2) + Creator(32) + BaseMint(32) + QuoteMint(32)
    // Offset = 16 + 8 + 2 + 32 + 32 + 32 = 122
    // BaseDecimals: offset 122 (u8)
    // QuoteDecimals: offset 123 (u8)
    try {
      event.baseDecimals = data.readUInt8(122);
      event.quoteDecimals = data.readUInt8(123);
    } catch (e) {
      console.error('Failed to merge CreatePool Event data', e);
    }
  }
}
