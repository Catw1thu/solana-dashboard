import { Controller, Get, Param, Query } from '@nestjs/common';
import { TokenService } from './token.service';

@Controller('/api/token')
export class TokenController {
  constructor(private readonly tokenService: TokenService) {}

  // GET /api/token/pool/:address
  @Get('pool/:address')
  async getPool(@Param('address') address: string) {
    return this.tokenService.getPool(address);
  }

  // GET /api/token/pools
  @Get('pools')
  async getPools(@Query('limit') limit?: string) {
    const limitNum = limit ? parseInt(limit, 10) : undefined;
    return this.tokenService.getPools(limitNum);
  }

  // GET /api/token/pools-stats
  @Get('pools-stats')
  async getPoolsWithStats(
    @Query('limit') limit?: string,
    @Query('offset') offset?: string,
  ) {
    const limitNum = limit ? parseInt(limit, 10) : undefined;
    const offsetNum = offset ? parseInt(offset, 10) : undefined;
    return this.tokenService.getPoolsWithStats(limitNum, offsetNum);
  }

  // GET /api/token/stats/:poolAddress
  @Get('stats/:poolAddress')
  async getPoolStats(@Param('poolAddress') poolAddress: string) {
    return this.tokenService.getPoolStats(poolAddress);
  }

  // GET /api/token/candles/:poolAddress
  @Get('candles/:poolAddress')
  async getCandles(
    @Param('poolAddress') poolAddress: string,
    @Query('resolution') resolution = '1m',
    @Query('from') from?: string,
    @Query('to') to?: string,
  ) {
    const resolutionMap = {
      '1m': '1 minute',
      '5m': '5 minutes',
      '15m': '15 minutes',
      '30m': '30 minutes',
      '1h': '1 hour',
      '4h': '4 hours',
      '1d': '1 day',
    };
    const dbResolution = resolutionMap[resolution] || '1 minute';
    const fromDate = from ? new Date(Number(from) * 1000) : undefined;
    const toDate = to ? new Date(Number(to) * 1000) : undefined;

    return this.tokenService.getOHLCV(
      poolAddress,
      dbResolution,
      fromDate,
      toDate,
    );
  }

  // GET /api/token/trades/:poolAddress
  @Get('trades/:poolAddress')
  async getTrades(
    @Param('poolAddress') poolAddress: string,
    @Query('limit') limit?: string,
  ) {
    const limitNum = limit ? parseInt(limit, 10) : undefined;
    return this.tokenService.getTrades(poolAddress, limitNum);
  }
}
