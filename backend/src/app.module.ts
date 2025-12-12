import { Module } from '@nestjs/common';
import { ConfigModule } from '@nestjs/config';
import { GrpcService } from './grpc/grpc.service';
import { AppController } from './app.controller';
import { AppService } from './app.service';
import { PumpSwapParser } from './dex-parsers/pumpSwap';
import { PumpFunParser } from './dex-parsers/pumpFun';

@Module({
  imports: [ConfigModule.forRoot()],
  controllers: [AppController],
  providers: [AppService, GrpcService, PumpSwapParser, PumpFunParser],
})
export class AppModule {}
