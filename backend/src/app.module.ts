import { Module } from '@nestjs/common';
import { ConfigModule } from '@nestjs/config';
import { GrpcService } from './grpc/grpc.service';
import { AppController } from './app.controller';
import { AppService } from './app.service';
import { PumpSwapParser } from './dex-parsers/pumpSwap';
import { PumpFunParser } from './dex-parsers/pumpFun';
import { RedisService } from './redis/redis.service';
import { DatabaseService } from './database/database.service';
import { TokenService } from './token/token.service';

import { TokenController } from './token/token.controller';
import { MetadataService } from './token/metadata.service';

@Module({
  imports: [ConfigModule.forRoot()],
  controllers: [AppController, TokenController],
  providers: [
    AppService,
    GrpcService,
    PumpSwapParser,
    PumpFunParser,
    RedisService,
    DatabaseService,
    TokenService,
    MetadataService,
  ],
})
export class AppModule {}
