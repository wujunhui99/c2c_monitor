# Runbook

## 本地启动

1. 准备配置：

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

## 常用验证

- 运行单元测试：

  ```bash
  make test
  ```

- 运行仓库自检：

  ```bash
  make doctor
  ```

- 日常提交前检查：

  ```bash
  make doctor
  ```

  这是每次提交前的硬性要求；如果失败，先修复再提交。

- 如果本次改动涉及第三方依赖，还必须确认：

  - 已补 happy path 测试
  - 已补至少一个 failure path 测试
  - 已做一次真实依赖请求验证

- 改动 `Dockerfile`、包结构或构建脚本后的额外检查：

  ```bash
  docker build -f Dockerfile .
  ```

  只要命中这类改动，这条也是提交前的硬性要求。

- 只启动后端进程：

  ```bash
  go run ./cmd/monitor -config config/config.yaml
  ```

- 如果改了版本说明：

  - 更新 `docs/releases.json`
  - 确保 `internal/appmeta/version.go` 里的当前版本存在于这个文件里

## 运行时接口

- `GET /api/v1/history`
- `GET /api/changelog`
- `GET /api/config`
- `POST /api/config`
- `GET /api/meta`
- `GET /api/alerts/status`
- `POST /api/alerts/reset`
- `GET /api/status`

## 常见问题

### 发布流程有哪些硬性要求

- 镜像发布前，CI 必须先跑前置验证，不能直接进入 build/push。
- 当前仓库的镜像发布 workflow 已经通过 `verify` job 先执行 `make doctor`。

### 后端无法启动

- 先看容器或进程的标准输出日志；当前应用日志默认是 JSON 行
- 本地可直接运行：

  ```bash
  go run ./cmd/monitor -config config/config.yaml | jq .
  ```

- 容器里可直接运行：

  ```bash
  docker logs <backend-container> | jq .
  ```

- 检查 `config/config.yaml` 是否存在
- 检查 `database.dsn` 是否可连通
- 如果是配置值不合法，启动时会直接失败，不会静默降级
- 如果是版本说明文件缺失或格式错误，启动时也会直接失败

### 如何查询结构化日志

- 按事件过滤 Forex 更新：

  ```bash
  docker logs <backend-container> | jq 'select(.event == "forex_updated")'
  ```

- 按事件过滤服务故障：

  ```bash
  jq 'select(.event == "service_down")' logs/service_down.log
  ```

- 按交易所过滤抓取失败：

  ```bash
  docker logs <backend-container> | jq 'select(.event == "exchange_fetch_failed" and .exchange == "Gate")'
  ```

### 数据库 schema 怎么初始化

- 当前不是启动时直接跑裸 `AutoMigrate`，而是执行显式版本迁移
- 已应用的 migration 会记录在 `schema_migrations`
- 新增表结构变化时，要追加新 migration，而不是直接修改启动流程

### 前端没数据

- 先确认 `GET /api/status` 是否返回 Forex 和各交易所状态
- 再确认 `frontend/js/config.js` 指向的是 `http://localhost:8001`
- 如果长时间范围为空，检查聚合表是否已经开始写入

### 修改了 docs 但不确定是否完整

- 跑 `make doctor`
- 检查 `AGENTS.md` 是否仍然只保留地图性质的信息
- 确认对应主题文档已经同步更新
