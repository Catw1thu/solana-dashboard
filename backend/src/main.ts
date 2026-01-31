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

  const app = await NestFactory.create(AppModule, {
    logger: ['error', 'warn'], // 仅保留错误和警告，过滤掉普通日志以方便排查问题
  });

  // Enable CORS - only allow local and this server's frontend
  app.enableCors({
    origin: [
      'http://localhost:3001',
      'http://127.0.0.1:3001',
      'http://173.249.210.151:3001', // This server's public IP
    ],
    methods: 'GET,HEAD,PUT,PATCH,POST,DELETE',
    credentials: true,
  });
  await app.listen(process.env.PORT ?? 3000);
}
bootstrap();
