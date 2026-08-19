# 2026-08 Reliability And Security

状态：已完成，待发布

## 背景

仓库已经具备文档、配置校验、migration、结构化日志和发布前验证，但管理接口与关键告警链路仍有线上风险：

- 写接口没有鉴权，CORS 范围过宽
- 邮件发送失败时仍会推进动态阈值，可能漏报
- 过期外汇缓存仍可参与告警计算
- OKX 先取单条广告再做金额过滤，可能漏掉可成交广告
- 运行时修改 Forex 周期不会重新调度
- 零金额档位无法通过告警重置接口
- 部分抓取失败会被同交易所其他成功档位掩盖

## 本次目标

- 为管理写接口增加 Bearer token 鉴权，并限制允许的跨域来源
- 让告警状态只在通知成功后推进，并为 SMTP 增加真实超时与取消
- 为 Forex 缓存增加最大年龄，过期时停止机会告警
- 修复 OKX 金额档位漏数据并补齐交易所适配器失败路径测试
- 让 C2C 与 Forex 调度响应运行时配置更新，避免轮次重叠
- 显示部分失败和空结果，不再把交易所错误标成健康
- 消除前端动态 HTML 注入点
- 增加 PR 验证、race、CodeQL、Dependabot、SBOM 和可复现镜像部署

## 配置迁移

- 新增 `app.admin_token`，至少 16 个字符；也可通过 `C2C_APP_ADMIN_TOKEN` 注入
- 新增 `app.allowed_origins`，只列出确实需要跨域访问 API 的前端来源
- 新增 `monitor.forex_max_age_hours`，控制缓存汇率参与告警的最长时间
- Compose 部署改用环境变量提供数据库密码和不可变镜像标签

## 验证标准

- `make doctor`
- `go test -race ./...`
- `go vet ./...`
- 后端与前端镜像均能成功构建
- Compose 配置可在提供必需环境变量后通过解析
- 未携带或携带错误 token 的管理写请求返回 `401`
- SMTP 失败不会创建动态阈值；成功后才更新内存和数据库
- 过期 Forex 缓存不会触发 C2C 告警
- OKX 第一条广告限额不匹配时仍能选中后续可成交广告

## 完成情况

- 管理写接口已增加 Bearer token 鉴权，CORS 已收敛到配置来源
- 告警、SMTP、Forex 新鲜度、热更新调度和交易所三态状态已按目标修复
- Binance、OKX、API、SMTP、告警一致性和数据库 migration 已补回归测试
- 前端动态状态和告警表已改用安全 DOM 渲染，管理员 token 只保存在当前标签页
- PR Verify、CodeQL、Dependabot、固定 action commit、race/vet、SBOM/provenance 已加入仓库
- Compose 已移除仓库内真实配置，改为显式 secret、健康检查、同源代理和完整 commit SHA 镜像
- 发布后把本文件移动到 `docs/exec-plans/completed/`
