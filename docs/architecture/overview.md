# Architecture Overview

## 系统目标

系统负责抓取多个交易所的 `USDT/CNY` C2C 买入价格，对照 `USD/CNY` 外汇汇率计算折价幅度，持久化历史数据，并在满足阈值时通过 SMTP 发送告警。

## 代码布局

- `cmd/monitor`
  - 负责进程启动、配置加载、依赖注入、HTTP 服务与优雅退出
- `config`
  - 负责配置文件读取、默认值、规范化与静态校验
- `internal/api`
  - 对外暴露历史查询、运行时配置、版本信息、版本变更记录、服务健康状态
- `internal/service`
  - 负责编排抓取循环、告警状态机、运行态服务状态
- `internal/infrastructure/exchange`
  - 交易所适配器。每个适配器只关心“如何取到标准化的 `PricePoint`”
- `internal/infrastructure/forex`
  - 汇率源适配器。具体源选择与降级要求见 `docs/architecture/external-dependencies.md`
- `internal/infrastructure/persistence/mysql`
  - MySQL DAO、版本化 schema migration、原始数据与聚合表读写
- `frontend`
  - 独立静态页面，只依赖后端 API

## 关键不变量

- 交易所名称统一使用标准写法：`Binance`、`Gate`、`OKX`
- 配置边界要尽早校验：端口、轮询周期、阈值、金额档位、交易所列表
- API 和配置层只处理规范化后的交易所名称，不依赖大小写约定
- 前端展示历史数据时，不硬编码交易所 response key，而是读取 `/api/meta` 返回的 `supported_exchanges` 和 `history_keys`
- 历史数据分三层存储：
  - `raw`：原始抓取数据
  - `hour`：小时聚合
  - `day`：天级聚合
- `MonitorService` 只编排流程，不直接知道底层 HTTP 或 SQL 细节
- 外部依赖按“强依赖权威源 / 可替换参考源 / 非核心依赖”分级处理，测试与降级策略不能一刀切
- 数据库初始化必须走显式 migration，并在 `schema_migrations` 里记录已应用版本
- 应用主日志统一输出 JSON 行，按 `event` 字段做查询

## 为什么这样改

这次不是给仓库塞更多“说明书”，而是把最容易漂移的几件事变清晰：

- 启动入口恢复为真实存在的 `cmd/monitor`
- 支持的交易所变成代码里的单一事实来源
- 配置错误在启动或更新时就报错，而不是让任务悄悄失效
- 文档从单文件 `PRD.md` 拆成按主题维护的结构
