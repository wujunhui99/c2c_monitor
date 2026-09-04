# Architecture Overview

## 系统目标

系统负责抓取多个交易所的 `USDT/CNY` C2C 买入价格，对照 `USD/CNY` 外汇汇率维护只降不升的全局标定价，持久化历史与告警状态，并在价格低于有效标定时通过 SMTP 发送告警。

## 代码布局

- `cmd/monitor`
  - 负责进程启动、配置加载、依赖注入、HTTP 服务与优雅退出
- `config`
  - 负责配置文件读取、默认值、规范化与静态校验
- `internal/api`
  - 对外暴露历史查询、运行时配置、版本信息、版本变更记录、服务健康状态
  - 管理写接口在路由层统一校验 Bearer token
  - CORS 只允许配置中声明的来源
- `internal/service`
  - 负责编排抓取循环、热更新调度、告警状态机、Forex 新鲜度和运行态服务状态
- `internal/infrastructure/exchange`
  - 交易所适配器。每个适配器只关心“如何取到标准化的 `PricePoint`”
- `internal/infrastructure/forex`
  - 汇率源适配器。具体源选择与降级要求见 `docs/architecture/external-dependencies.md`
- `internal/infrastructure/notifier`
  - 通过带上下文超时的 TLS SMTP 会话发送邮件，并拒绝邮件头换行注入
- `internal/infrastructure/persistence/mysql`
  - MySQL DAO、版本化 schema migration、查询索引、原始数据与聚合表读写
- `frontend`
  - 独立静态页面，只依赖后端 API；动态文本默认通过 DOM `textContent` 渲染
- `deploy/k8s`
  - 生产环境单一事实来源：独立 namespace、MySQL StatefulSet、应用 Deployments、Traefik Ingress
- `scripts/deploy-k3s.sh`
  - 负责首次密钥生成、Kubernetes Secret 同步、不可变镜像渲染、rollout、证书和公网冒烟验证

## 关键不变量

- 交易所名称统一使用标准写法：`Binance`、`Bitget`、`Gate`、`OKX`
- 配置边界要尽早校验：端口、轮询周期、金额档位、交易所列表
- 管理写接口只有 `POST /api/config`、`POST /api/alerts/benchmark` 和 `POST /api/alerts/reset`，必须经过 Bearer token 鉴权
- 管理 token 不通过读取接口返回，前端只在当前浏览器标签页会话中保存
- API 和配置层只处理规范化后的交易所名称，不依赖大小写约定
- 前端展示历史数据时，不硬编码交易所 response key，而是读取 `/api/meta` 返回的 `supported_exchanges` 和 `history_keys`
- C2C 和 Forex 调度读取配置快照，运行时配置更新会唤醒两个调度器重新计时
- 单个 C2C 轮次完成前不会启动下一轮；轮次内部有并发上限
- Forex 超过配置的最大年龄后，`/readyz` 返回失败，C2C 价格继续采集，但机会告警停止
- 交易所状态必须区分全成功、部分成功和全失败，不能用一个成功金额档位掩盖其余错误
- 全局默认标定持久化到 `alert_benchmarks`，金额档位覆盖持久化到 `alert_benchmark_overrides`
- 每个档位按 `min(默认标定, 档位覆盖, 当前 Forex)` 只向下收敛
- 市场新低状态只能在通知成功后推进；数据库重置失败时不能先删除内存状态
- 历史数据分三层存储：
  - `raw`：原始抓取数据
  - `hour`：小时聚合
  - `day`：天级聚合
- `MonitorService` 只编排流程，不直接知道底层 HTTP 或 SQL 细节
- 外部依赖按“强依赖权威源 / 可替换参考源 / 非核心依赖”分级处理，测试与降级策略不能一刀切
- 数据库初始化必须走显式 migration，并在 `schema_migrations` 里记录已应用版本
- 面向历史查询的复合索引只能通过追加 migration 发布，不能依赖已有环境重新执行旧 migration
- 应用主日志统一输出 JSON 行，按 `event` 字段做查询
- `/healthz` 只检查进程存活，`/readyz` 检查 Forex 数据是否可用于业务计算
- 生产域名固定为 `c2c.agenticim.xyz`，Ingress 使用 `traefik` 和 `letsencrypt-prod`
- 生产镜像必须使用完整 commit SHA；backend 使用 `Recreate`，避免部署窗口同时运行两套监控循环
- 生产 secret 只存在 `/opt/c2c-monitor/secrets.env` 和 `c2c-monitor` namespace 的 Kubernetes Secret

## 为什么这样改

这次不是给仓库塞更多“说明书”，而是把最容易漂移的几件事变清晰：

- 启动入口恢复为真实存在的 `cmd/monitor`
- 支持的交易所变成代码里的单一事实来源
- 配置错误在启动或更新时就报错，而不是让任务悄悄失效
- 文档从单文件 `PRD.md` 拆成按主题维护的结构
- 管理面、外部依赖和部署路径有了明确的失败边界
