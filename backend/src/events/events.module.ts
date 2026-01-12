import { Module } from '@nestjs/common';
import { EventsGateway } from './events.gateway';
import { BatcherService } from './batcher.service';

@Module({
  providers: [EventsGateway, BatcherService],
  exports: [EventsGateway, BatcherService],
})
export class EventsModule {}
