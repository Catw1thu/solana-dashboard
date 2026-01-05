-- 安装超表插件
CREATE EXTENSION IF NOT EXISTS timescaledb;
-- 查看超表
SELECT * FROM pg_extension WHERE extname = 'timescaledb';
-- 开启超表
SELECT create_hypertable('"Trade"', 'time');
-- 查看所有超表
SELECT * FROM timescaledb_information.hypertables;