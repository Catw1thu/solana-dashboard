import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
import { Logger } from '@nestjs/common';
import * as util from 'util';

async function bootstrap() {
  const logger = new Logger('Global');

  // Monkey-patch console to use NestJS Logger
  console.log = (...args) => logger.log(util.format(...args));
  console.error = (...args) => logger.error(util.format(...args));
  console.warn = (...args) => logger.warn(util.format(...args));
  console.debug = (...args) => logger.debug(util.format(...args));

  const app = await NestFactory.create(AppModule);
  await app.listen(process.env.PORT ?? 3000);
}
bootstrap();
