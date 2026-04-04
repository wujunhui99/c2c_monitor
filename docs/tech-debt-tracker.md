# Tech Debt Tracker

## 高优先级

- 还没有 CI 自动执行 `make doctor`
- `cmd/monitor` 只有构建验证，缺少端到端启动测试
- 运行时更新配置不会落盘，重启后会回到文件配置

## 中优先级

- 历史 API 仍然返回固定的交易所键，前后端契约没有单独测试
- 日志仍以标准库 `log` 为主，缺少结构化字段与统一查询方式
- MySQL `AutoMigrate` 适合当前阶段，但长期可能需要显式 migration

## 低优先级

- `frontend/` 仍然是原生静态页面，缺少更正式的构建流程
- 部署说明分散在多个 README 中，后续可以再统一索引
