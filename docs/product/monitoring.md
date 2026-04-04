# Monitoring Product Spec

## 产品目标

帮助用户持续观察 `USDT/CNY` C2C 买入价格是否低于 `USD/CNY` 外汇参考价，并在出现折价机会时尽快收到通知，同时支持事后查看历史走势。

## 当前范围

### 数据采集

- 交易所：`Binance`、`Gate`、`OKX`
- 资产：`USDT/CNY`
- 方向：用户 `BUY`
- 采集粒度：
  - C2C：按 `target_amounts` 轮询
  - Forex：按小时刷新

### 历史数据

- 原始表保存每次抓取结果
- 小时表和天表做聚合，减少长时间范围查询的扫描量
- `GET /api/v1/history` 自动根据时间范围切换数据源
- 前端使用 `GET /api/meta` 返回的 `supported_exchanges` 和 `history_keys` 来决定如何渲染历史曲线，不再硬编码交易所 key

### 告警

- 初次触发条件：
  - `spread = (forex - c2c_price) / forex * 100`
  - `spread >= alert_threshold_percent`
- 首次触发后进入动态低价跟踪
- 后续只有出现更低价格才继续触发 `Lower` 告警
- 告警状态持久化到 `alert_states`，重启后恢复

### 运行时配置

- `GET /api/config` 返回当前监控配置
- `GET /api/meta` 返回当前服务版本以及前端渲染所需的交易所元数据
- `GET /api/changelog` 返回版本变更记录
- `POST /api/config` 更新运行中配置
- 当前更新只影响内存态，不回写 `config.yaml`

## 明确非目标

- 不支持前端直接修改持久化配置文件
- 不做自动交易或自动下单
- 不做多币种、多法币、多方向的一般化抽象

## 对后续迭代的要求

- 如果新增交易所，先更新交易所单一事实来源和文档，再接入适配器
- 如果改动告警策略，必须同步更新本文件与相关测试
- 如果新增更复杂的后台任务，优先在 `docs/exec-plans/active/` 写执行计划
