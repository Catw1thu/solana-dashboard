import { Module } from '@nestjs/common';
import { ConfigModule } from '@nestjs/config';
import { GrpcService } from './grpc/grpc.service';
import { AppController } from './app.controller';
import { AppService } from './app.service';

@Module({
  imports: [ConfigModule.forRoot()],
  controllers: [AppController],
  providers: [AppService, GrpcService],
})
export class AppModule {}
