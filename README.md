# C2C Monitor

`c2c_monitor` 用来持续抓取 `USDT/CNY` C2C 价格、按金额档位维护只降不升的告警标定价，并在 C2C 价格创新低时触发邮件告警，同时提供历史走势 API 给前端展示。

## Quick Start

1. 准备本地 MySQL 环境变量并启动数据库：

   ```bash
   cp .env.example .env
   # Fill MYSQL_ROOT_PASSWORD and MYSQL_PASSWORD in .env.
   docker compose up -d mysql
   ```

2. 复制配置文件，填写随机管理员 token、数据库 DSN、SMTP 凭据和收件人：

   ```bash
   cp config/config.yaml.example config/config.yaml
   # app.admin_token must contain at least 16 characters.
   ```

   也可以用 `C2C_APP_ADMIN_TOKEN`、`C2C_DATABASE_DSN` 和
   `C2C_NOTIFICATION_EMAIL_*` 环境变量覆盖配置文件里的敏感值。

3. 启动后端：

   ```bash
   make start-backend
   ```

4. 启动前端：

   ```bash
   make start-frontend
   ```

5. 访问：
   - 前端：`http://localhost:8080`
   - 后端：`http://localhost:8001`
   - 存活检查：`http://localhost:8001/healthz`
   - 业务就绪检查：`http://localhost:8001/readyz`

管理员 token 只用于保存运行时配置、下调告警标定价和重置市场新低。前端把它保存在当前标签页的
`sessionStorage`，关闭标签页后清除。

## 常用命令

- `make build`：编译后端
- `make test`：运行 Go 单元测试
- `make doctor`：运行仓库自检，验证文档骨架、测试和构建
- `go test -race ./...`：运行并发竞态检查
- `go vet ./...`：运行 Go 静态检查
- `go run ./cmd/monitor -config config/config.yaml`：直接运行后端

## 文档入口

- [`AGENTS.md`](AGENTS.md)：给人和 agent 的仓库地图，保持简短
- [`docs/README.md`](docs/README.md)：文档总索引
- [`docs/product/monitoring.md`](docs/product/monitoring.md)：产品与行为约束
- [`docs/architecture/overview.md`](docs/architecture/overview.md)：架构边界与代码布局
- [`docs/operations/runbook.md`](docs/operations/runbook.md)：运行、调试、排障
- [`docs/exec-plans/active/2026-08-reliability-security.md`](docs/exec-plans/active/2026-08-reliability-security.md)：当前迭代计划
- [`docs/exec-plans/active/2026-08-k3s-production-deploy.md`](docs/exec-plans/active/2026-08-k3s-production-deploy.md)：生产部署计划

## 设计原则

这个仓库不再维护一个不断膨胀的 `PRD.md`。我们改成：

- `AGENTS.md` 只保留导航和稳定规则
- `docs/` 作为仓库内记录系统，按主题分层
- 计划、运行说明和技术债务都进入版本控制，方便后续持续迭代

## 版本

- 当前版本由后端统一提供，定义在 `internal/appmeta/version.go`
- 前端通过 `/api/meta` 读取并显示版本号
- 每次功能更新后，都要同步更新版本号和 `docs/releases.json`
