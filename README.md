# C2C Monitor

`c2c_monitor` 用来持续抓取 `USDT/CNY` C2C 价格、对照 `USD/CNY` 外汇参考价计算价差，并在满足阈值时触发邮件告警，同时提供历史走势 API 给前端展示。

## Quick Start

1. 复制配置文件并填入真实数据库和 SMTP 信息：

   ```bash
   cp config/config.yaml.example config/config.yaml
   ```

2. 启动后端：

   ```bash
   make start-backend
   ```

3. 启动前端：

   ```bash
   make start-frontend
   ```

4. 访问：
   - 前端：`http://localhost:8080`
   - 后端：`http://localhost:8001`

## 常用命令

- `make build`：编译后端
- `make test`：运行 Go 单元测试
- `make doctor`：运行仓库自检，验证文档骨架、测试和构建
- `go run ./cmd/monitor -config config/config.yaml`：直接运行后端

## 文档入口

- [`AGENTS.md`](AGENTS.md)：给人和 agent 的仓库地图，保持简短
- [`docs/README.md`](docs/README.md)：文档总索引
- [`docs/product/monitoring.md`](docs/product/monitoring.md)：产品与行为约束
- [`docs/architecture/overview.md`](docs/architecture/overview.md)：架构边界与代码布局
- [`docs/operations/runbook.md`](docs/operations/runbook.md)：运行、调试、排障
- [`docs/exec-plans/active/2026-04-harness-foundation.md`](docs/exec-plans/active/2026-04-harness-foundation.md)：当前迭代计划

## 设计原则

这个仓库不再维护一个不断膨胀的 `PRD.md`。我们改成：

- `AGENTS.md` 只保留导航和稳定规则
- `docs/` 作为仓库内记录系统，按主题分层
- 计划、运行说明和技术债务都进入版本控制，方便后续持续迭代
