# C2C 溢价/折价监测系统 PRD（与当前代码实现一致）

- 项目名称：`c2c_monitor`
- 版本：`v1.1.0`
- 更新时间：`2026-02-25`
- 状态：开发中

## 1. 项目目标

监控 C2C 市场中 `USDT/CNY` 价格与 `USD/CNY` 外汇参考价之间的价差，在出现折价机会时触发告警，并提供历史走势可视化。

核心目标：
- 稳定采集 Binance、OKX 的 C2C 买入价格。
- 结合外汇汇率进行阈值告警。
- 支持动态阈值（触发后关注更低价）并可重置。
- 前后端分离，后端仅提供 API，前端独立静态部署。

## 2. 当前实现范围

### 2.1 技术栈

- 后端：Go + Gin + GORM
- 数据库：MySQL 8
- 前端：原生 HTML/CSS/JS + ECharts（独立静态站点）
- 汇率源：`open.er-api.com`（代码中适配器名称保留为 `YahooForexAdapter`，但实际请求 OpenER）
- 通知：SMTP 邮件

### 2.2 交易所支持

- 已实现：`Binance`、`OKX`
- 未实现：Gate（文档历史版本提及，但当前代码未接入）

## 3. 功能需求（按现有实现）

### 3.1 数据采集

- C2C 采集对象：`USDT/CNY`
- 方向：`BUY`（用户买入 USDT）
- 轮询间隔：
  - `c2c_interval_minutes`（默认 6）
  - 每轮额外随机抖动 `0~60s`
- 金额档位：`target_amounts`（默认 `[0,30,50,200,500,1000]`）
  - `0` 代表最低价（不限制金额）
- 取价策略：每个交易所、每个档位仅取 `Top 1`（`rank=1`）并入库
- 失败重试：单次抓取最多重试 3 次，每次间隔 90 秒

### 3.2 外汇采集

- 货币对：`USD -> CNY`
- 轮询间隔：`forex_interval_hours`（默认 1）
- 失败兜底：抓取失败时尝试读取 DB 最新汇率作为缓存值

### 3.3 告警逻辑

对每条 `rank=1` C2C 价格执行：

1. 初次触发条件：
   - `spread = (forex - c2c_price) / forex * 100`
   - `spread >= alert_threshold_percent` 时触发（默认阈值 0.1）
   - 初次触发受 30 分钟冷却保护（按 `exchange-side-amount` 维度）

2. 动态低价模式：
   - 某市场首次触发后，记录“触发最低价”
   - 后续只有当价格再次低于该记录价时继续触发（`Lower` 告警）

3. 重置：
   - 可通过 API 按 `exchange/side/amount` 重置该市场动态阈值

### 3.4 服务无状态化（本次更新）

为避免重启后丢失动态阈值，新增持久化：

- 启动时：从 `alert_states` 表恢复各市场触发最低价与最后告警时间。
- 触发告警时：`upsert` 写入 `alert_states`。
- 重置阈值时：同步删除 `alert_states` 对应记录。

这保证了“最低触发价状态”不依赖单机内存，服务重启后可恢复。

### 3.5 服务健康状态

- 维护交易所与汇率服务状态（`OK/Error`、错误消息、最后检查时间）
- 状态从 `OK -> Error` 时发送一次故障告警邮件
- 恢复后状态回到 `OK`

### 3.6 历史查询分层存储（新增）

为减少长时间范围查询的 DB 扫描和前后端传输量，系统采用三层历史数据：

- `raw`：原始采集数据（`c2c_prices`、`forex_rates`）
- `hour`：小时级聚合数据（`c2c_prices_hourly`、`forex_rates_hourly`）
- `day`：天级聚合数据（`c2c_prices_daily`、`forex_rates_daily`）

聚合规则：

- C2C：每个 bucket 保留最低价（`MIN` 语义，按 `exchange/symbol/fiat/side/target_amount/rank` 维度）
- Forex：每个 bucket 保留最新值（同 bucket 后写覆盖）

读取路由（由后端自动选择）：

- `range=1d`：读取 `raw`
- `range=7d` 或 `30d`：读取 `hour`
- `range=all`：读取 `day`

## 4. 数据模型（MySQL）

### 4.1 `c2c_prices`

字段（主要）：
- `id`、`created_at`
- `exchange`、`symbol`、`fiat`、`side`
- `target_amount`、`rank`、`price`
- `merchant_id`、`pay_methods`
- `min_amount`、`max_amount`、`available_amount`

索引：
- 复合索引 `idx_query(exchange, side, target_amount, rank, created_at)`

### 4.2 `merchants`

字段（主要）：
- `id`、`exchange`、`merchant_id`、`nick_name`、`created_at`、`updated_at`

索引：
- 唯一索引 `idx_merchant(exchange, merchant_id)`

### 4.3 `forex_rates`

字段：
- `id`、`created_at`、`source`、`pair`、`rate`

索引：
- `idx_time(created_at)`

### 4.4 `alert_states`（新增）

字段：
- `id`
- `exchange`
- `side`
- `target_amount`
- `trigger_price`
- `last_alert_at`
- `created_at`、`updated_at`

索引：
- 唯一索引 `idx_alert_state(exchange, side, target_amount)`

用途：
- 持久化动态阈值状态，支持服务重启恢复。

### 4.5 `c2c_prices_hourly` / `c2c_prices_daily`（新增）

字段（主要）：
- `bucket_time`
- `exchange`、`symbol`、`fiat`、`side`
- `target_amount`、`rank`
- `price`（bucket 内最低价）
- `created_at`、`updated_at`

索引：
- 唯一索引（bucket + 业务维度）用于 `upsert` 聚合更新。

### 4.6 `forex_rates_hourly` / `forex_rates_daily`（新增）

字段（主要）：
- `bucket_time`
- `pair`、`source`
- `rate`（bucket 最新值）
- `created_at`、`updated_at`

索引：
- 唯一索引 `(bucket_time, pair)` 用于 `upsert` 聚合更新。

## 5. 架构设计（与代码一致）

- `cmd/monitor/main.go`
  - 负责配置加载、依赖注入、启动监控协程、启动 HTTP API
- `internal/service/monitor_service.go`
  - 业务编排（抓取、存储、告警、状态管理）
- `internal/infrastructure/exchange/*`
  - Binance/OKX 适配器
- `internal/infrastructure/forex/yahoo.go`
  - OpenER 汇率适配器
- `internal/infrastructure/persistence/mysql/repository.go`
  - MySQL 仓储实现
- `internal/api/*`
  - Gin 路由与 Handler
- `frontend/*`
  - 独立 SPA，直接调用后端 API

## 6. API 定义（当前）

### 6.1 `GET /api/v1/history`

Query:
- `amount`（必填）
- `range`：`1d | 7d | 30d | all`（未命中时默认按 1d）

返回：`forex/binance/okx` 三条序列。

数据源选择（后端内部逻辑）：

- `1d`：原始表
- `7d/30d`：小时聚合表
- `all`：天聚合表

### 6.2 `GET /api/config`

返回当前监控配置。

### 6.3 `POST /api/config`

更新运行中配置（当前实现为内存更新，不写回 `config.yaml`）。

### 6.4 `GET /api/alerts/status`

返回动态阈值状态映射：`{ "Exchange-Side-Amount": trigger_price }`。

### 6.5 `POST /api/alerts/reset`

按 `exchange/side/amount` 重置动态阈值。

### 6.6 `GET /api/status`

返回外部服务健康状态映射。

## 7. 配置文件

示例路径：`config/config.yaml`

关键字段：
- `app.port`
- `monitor.c2c_interval_minutes`
- `monitor.forex_interval_hours`
- `monitor.alert_threshold_percent`
- `monitor.target_amounts`
- `monitor.exchanges`
- `database.dsn`
- `notification.email.*`

## 8. 前后端分离与 Docker 部署结论

### 8.1 当前前后端分离是否有问题

当前逻辑总体可行：
- 后端仅提供 API，不耦合前端静态文件。
- 前端独立部署，通过 `AppConfig.apiBaseUrl` 访问 API。
- 后端已启用 CORS（当前为 `AllowAllOrigins=true`）。

需要注意：
- 前端 `config.js` 默认写死 `http://localhost:8001`，生产环境建议改为可注入配置。
- CORS 全开放适合开发，不建议生产直接使用。

### 8.2 前后端两个 Docker 镜像是否更好

是，更推荐两个镜像：
- `backend` 镜像：Go API + 采集/告警逻辑
- `frontend` 镜像：Nginx 托管静态资源

优点：
- 可独立扩缩容与发布
- 镜像职责清晰
- 更符合前后端分离标准实践

### 8.3 是否需要修改前后端分离方案

建议小幅优化，而非推翻：

1. 部署层建议：
- 使用 Nginx（前端容器）反代 `/api` 到后端容器
- 前端请求改为同域 `/api` 路径，减少 CORS 复杂度

2. 配置层建议：
- 前端 `apiBaseUrl` 支持环境注入（启动脚本或构建时替换）
- 后端 CORS 改为可配置白名单（生产限制来源）

3. 数据层建议：
- 保持 MySQL 独立容器，并持久化数据卷

## 9. 里程碑（下一步建议）

- M1：补齐 `backend/frontend` Dockerfile 与完整 `docker-compose`（含 mysql）
- M2：前端 `apiBaseUrl` 环境注入 + Nginx `/api` 反向代理
- M3：后端 CORS 白名单配置化
- M4：`POST /api/config` 支持落盘并安全热更新（可选）
