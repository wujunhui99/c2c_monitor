# AGENTS.md

这个文件只做仓库地图，不做百科全书。

## Start Here

- 产品行为与当前范围：`docs/product/monitoring.md`
- 架构边界与代码布局：`docs/architecture/overview.md`
- 外部依赖策略与测试要求：`docs/architecture/external-dependencies.md`
- 运行、调试、排障：`docs/operations/runbook.md`
- 版本变更记录：`docs/releases.json`
- 当前活跃计划：`docs/exec-plans/active/2026-04-harness-foundation.md`
- 已知技术债务：`docs/tech-debt-tracker.md`

## Repo Map

- `cmd/monitor`：后端启动入口
- `config`：配置加载、规范化与校验
- `internal/api`：HTTP 路由与 handler
- `internal/service`：监控编排、告警逻辑、运行态状态
- `internal/infrastructure/exchange`：交易所抓取适配器
- `internal/infrastructure/forex`：外汇汇率抓取适配器
- `internal/infrastructure/persistence/mysql`：MySQL 仓储与聚合表写入
- `frontend`：独立静态前端
- `deploy`：Compose / K8s 部署资产
- `scripts/doctor.sh`：仓库自检入口

## Working Rules

- 保持这个文件简短。更深的说明放到 `docs/`。
- 变更产品行为时，同时更新 `docs/product/monitoring.md`。
- 变更启动、部署、排障方式时，同时更新 `docs/operations/runbook.md`。
- 变更模块边界、目录职责或关键不变量时，同时更新 `docs/architecture/overview.md`。
- 变更第三方依赖、抓取源、通知通道或外部 API 策略时，同时更新 `docs/architecture/external-dependencies.md`。
- 变更用户可见功能或版本号时，同时更新 `docs/releases.json`。
- 新迭代开始前，优先在 `docs/exec-plans/active/` 新建或更新执行计划。
- 每次提交前都必须运行 `make doctor`；如果失败，先修复再提交。
- 如果改动了 `Dockerfile`、包结构或构建脚本，提交前还必须额外运行一次 `docker build -f Dockerfile .`。
- 涉及镜像发布的 CI 必须保留前置验证，不能直接进入镜像构建或推送步骤。
- 新增或修改外部依赖时，测试不能只覆盖 happy path；必须至少覆盖一个 failure path。
- `docs/tech-debt-tracker.md` 只保留尚未完成的技术债；某项已经修复后，要在同一批变更里把它从文档删除。
- 不要重新引入新的顶层 `PRD.md`；让 `docs/` 保持单一事实来源。

## Mechanical Invariants

- 支持的交易所只有：`Binance`、`Gate`、`OKX`
- 配置加载阶段会把交易所名称规范化为上面的标准写法，并拒绝不支持的值
- `make doctor` 必须通过，至少覆盖：
  - 核心 docs 骨架存在
  - `AGENTS.md` 保持短小
  - `go test ./...`
  - `go build ./cmd/monitor`

## Common Commands

- `make build`
- `make test`
- `make doctor`
- `make start-backend`
- `make start-frontend`
- `go run ./cmd/monitor -config config/config.yaml`
