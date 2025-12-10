import { Injectable } from '@nestjs/common';

export interface RaydiumSwapEvent {
  logType: number;
  amountIn: bigint;
  amountOut: bigint;
  minOut: bigint;
  direction: bigint;
  userSourceTokenAccount: bigint;
  poolCoinTokenAccount: bigint;
  poolPcTokenAccount: bigint;
  userDestinationTokenAccount: bigint;
}

export class TransactionFormatter {
  public decodeRaydiumLog(base64Data: string): RaydiumSwapEvent | null {
    try {
      const buffer = Buffer.from(base64Data, 'base64');
      // Raydium 的 ray_log 并不总是 Swap 事件，可能是 Init, Deposit 等
      // Swap 事件的 log_type 通常非 0 (视具体版本而定)，但我们先解析出来再看

      // 这里的 Layout 参考自 Raydium SDK
      // Offset | Size | Name
      // 0      | 1    | log_type
      // 1      | 8    | amount_in (u64)
      // 9      | 8    | amount_out (u64)
      // 17     | 8    | min_out (u64)
      // 25     | 8    | direction (u64)
      // ... 后面还有，但我们简历项目主要关注前几个核心数据
      if (buffer.length < 33) {
        // 数据太短，肯定不是我们要的 Swap Log
        console.log('Raydium log data too short to be a valid Swap event.');
        return null;
      }

      const logType = buffer.readUInt8(0);
      const amountIn = buffer.readBigUInt64LE(1);
      const amountOut = buffer.readBigUInt64LE(9);
      const minOut = buffer.readBigUInt64LE(17);
      const direction = buffer.readBigUInt64LE(25);

      // 简单的过滤器：如果进出金额都是0，可能不是有效 Swap
      if (amountIn === BigInt(0) && amountOut === BigInt(0)) {
        console.log('Raydium log indicates zero amount swap, ignoring.');
        return null;
      }

      return {
        logType,
        amountIn,
        amountOut,
        minOut,
        direction,
        // 后面的暂时略过，为了保持代码简单
        userSourceTokenAccount: BigInt(0),
        poolCoinTokenAccount: BigInt(0),
        poolPcTokenAccount: BigInt(0),
        userDestinationTokenAccount: BigInt(0),
      };
    } catch (error) {
      console.error(`Failed to decode Raydium log: ${error}`);
      return null;
    }
  }
}
