# Tech Debt Tracker

## 中优先级

- 管理面仍使用单个部署级共享 token，缺少独立账号、轮换流程和操作审计
- 数据库 migration 测试使用 SQLite，尚缺一次真实 MySQL 容器集成测试

## 低优先级

- `frontend/` 仍然是原生静态页面，缺少更正式的构建流程
- ECharts 仍从公共 CDN 加载；虽然已固定版本并启用 SRI，但离线部署需要改为自托管资源
- 部署说明分散在多个 README 中，后续可以再统一索引
- Kubernetes 目录仍是占位说明，当前只有 Compose 路径可用于生产
