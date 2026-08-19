# Runbook

## 本地启动

1. 准备并启动 MySQL：

   ```bash
   cp .env.example .env
   # 填写 MYSQL_ROOT_PASSWORD 和 MYSQL_PASSWORD
   docker compose up -d mysql
   ```

2. 准备应用配置：

   ```bash
   cp config/config.yaml.example config/config.yaml
   # 填写 app.admin_token、database.dsn、SMTP 凭据和收件人
   ```

3. 启动后端：

   ```bash
   make start-backend
   ```

4. 启动前端：

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
  go test -race ./...
  go vet ./...
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

- 如果改了 workflow：

  ```bash
  ruby -e 'require "yaml"; ARGV.each { |path| YAML.parse_file(path); puts "ok #{path}" }' .github/workflows/*.yml .github/dependabot.yml
  ```

## 运行时接口

- `GET /api/v1/history`
- `GET /api/changelog`
- `GET /api/config`
- `POST /api/config`
- `GET /api/meta`
- `GET /api/alerts/status`
- `POST /api/alerts/reset`
- `GET /api/status`
- `GET /healthz`
- `GET /readyz`

### 管理写接口

`POST /api/config` 和 `POST /api/alerts/reset` 都要求：

```text
Authorization: Bearer <app.admin_token>
```

示例：

```bash
curl -fsS \
  -H "Authorization: Bearer $C2C_APP_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "c2c_interval_minutes": 6,
    "forex_interval_hours": 1,
    "forex_max_age_hours": 6,
    "alert_threshold_percent": 0.1,
    "target_amounts": [0, 30, 50, 200, 500, 1000],
    "exchanges": ["Binance", "Gate", "OKX"]
  }' \
  http://127.0.0.1:8001/api/config
```

运行时配置只更新内存，不修改 `config.yaml`。C2C 和 Forex 调度器会立即按新周期重新计时。

### 健康检查

- `/healthz`：只要 HTTP 进程可响应就返回 `200`
- `/readyz`：只有当前存在未超过 `forex_max_age_hours` 的 Forex 参考价才返回 `200`

部署探活用 `/healthz`；接流量、发布验收和业务告警用 `/readyz`。

## 常见问题

### 发布流程有哪些硬性要求

- PR 和 `main` 推送会运行 Verify 与 CodeQL。
- 镜像发布前必须通过 doctor、race、vet 和前端 JavaScript 语法检查。
- 后端与前端镜像都发布 `latest` 和完整 40 位 commit SHA 标签，并生成 SBOM 与 provenance。
- 生产部署只接受完整 commit SHA 标签，不使用可变 `latest` 标签。
- 部署 workflow 会从该 SHA checkout Compose/Nginx 资产，确保它们与镜像来自同一提交。
- 手动部署 workflow 需要配置 `production` environment、SSH/GHCR secrets 和部署变量。

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
- 检查 `app.admin_token` 是否至少 16 个字符，或 `C2C_APP_ADMIN_TOKEN` 是否已注入
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
- 再确认 `GET /readyz` 是否因 Forex 不可用或过期返回 `503`
- 再确认 `frontend/js/config.js` 指向的是 `http://localhost:8001`
- 如果长时间范围为空，检查聚合表是否已经开始写入

### 管理操作返回 401

- 确认请求头使用精确的 `Bearer <token>` 格式
- 确认前端设置页中的 token 与后端启动时的 `app.admin_token` 一致
- token 被拒绝后，前端会从 `sessionStorage` 清除旧值并要求重新输入

### 服务显示 Degraded

- `Degraded` 表示同一交易所至少一个金额档位有数据，但另一些档位失败或返回空结果
- 展开前端状态详情，查看具体金额档位和错误
- 如果所有档位都没有数据，状态会升级为 `Error`

### Forex 拉取失败

- 查看 `forex_fetch_failed`、`forex_cache_used` 和 `forex_cache_stale` 事件
- 主源和备用源都失败时，只会使用仍在最大年龄内的数据库缓存
- 缓存过期后 `/readyz` 返回 `503`，C2C 历史价格继续采集，但暂停机会告警

### Compose 部署检查

```bash
cd deploy/compose
docker compose config --quiet
docker compose ps
curl -fsS http://127.0.0.1:8080/healthz
curl -fsS http://127.0.0.1:8080/readyz
```

- `.env` 和 `config.yaml` 都不应提交到 Git
- `IMAGE_TAG` 必须是镜像发布 workflow 生成的完整 commit SHA
- 前端默认只绑定 `127.0.0.1:8080`

### 修改了 docs 但不确定是否完整

- 跑 `make doctor`
- 检查 `AGENTS.md` 是否仍然只保留地图性质的信息
- 确认对应主题文档已经同步更新
