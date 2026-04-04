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

- 只启动后端进程：

  ```bash
  go run ./cmd/monitor -config config/config.yaml
  ```

## 运行时接口

- `GET /api/v1/history`
- `GET /api/config`
- `POST /api/config`
- `GET /api/alerts/status`
- `POST /api/alerts/reset`
- `GET /api/status`

## 常见问题

### 后端无法启动

- 先看 `logs/backend.log`
- 检查 `config/config.yaml` 是否存在
- 检查 `database.dsn` 是否可连通
- 如果是配置值不合法，启动时会直接失败，不会静默降级

### 前端没数据

- 先确认 `GET /api/status` 是否返回 Forex 和各交易所状态
- 再确认 `frontend/js/config.js` 指向的是 `http://localhost:8001`
- 如果长时间范围为空，检查聚合表是否已经开始写入

### 修改了 docs 但不确定是否完整

- 跑 `make doctor`
- 检查 `AGENTS.md` 是否仍然只保留地图性质的信息
- 确认对应主题文档已经同步更新
