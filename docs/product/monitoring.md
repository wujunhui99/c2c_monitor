# Monitoring Product Spec

## 产品目标

帮助用户持续观察 `USDT/CNY` C2C 买入价格是否低于 `USD/CNY` 外汇参考价，并在出现折价机会时尽快收到通知，同时支持事后查看历史走势。

## 当前范围

### 数据采集

- 交易所：`Binance`、`Gate`、`OKX`
- 资产：`USDT/CNY`
- 方向：用户 `BUY`
- 采集粒度：
  - C2C：按 `target_amounts` 轮询
  - Forex：按小时刷新
- C2C 和 Forex 周期在运行时更新后立即重新调度
- 同一种采集任务不会并发重叠执行
- 单次 C2C 轮次会限制并发抓取数，并对短暂上游错误做有限次指数退避重试

### 历史数据

- 原始表保存每次抓取结果
- 小时表和天表做聚合，减少长时间范围查询的扫描量
- `GET /api/v1/history` 自动根据时间范围切换数据源
- 前端使用 `GET /api/meta` 返回的 `supported_exchanges` 和 `history_keys` 来决定如何渲染历史曲线，不再硬编码交易所 key

### 告警

- 全局标定价首次默认为当前可用 `USD/CNY` Forex 汇率，并持久化到 `alert_benchmarks`
- 每次使用标定价前执行 `benchmark = min(benchmark, current_forex)`：
  - Forex 上涨时标定价不变
  - Forex 下跌时标定价自动下调并持久化
- 每个 `target_amount` 可单独保存一个更低的档位标定价
- 未设置覆盖的档位继承全局默认标定；已设置档位使用 `min(global_benchmark, amount_override)`
- 前端可通过 `POST /api/alerts/benchmark` 手动下调标定价：
  - 新值必须大于 `0`
  - 新值必须严格低于当前 Forex
  - 新值必须严格低于所选档位当前有效标定价，不能手动抬高
  - `target_amount` 为空时修改全局默认标定，否则只修改指定金额档位
- 每个交易所、方向和金额档位独立维护最近一次成功告警价格
- 实际比较值为 `min(amount_benchmark, last_successful_alert_price)`
- 当前 C2C 价格严格低于实际比较值时发送邮件
- 只有 SMTP 发送成功后，才把该市场的最近告警价格推进到当前 C2C 价格
- SMTP 失败时市场新低状态不推进，后续轮次仍可重试
- `notification.email.enabled=false` 时不尝试发送邮件，也不推进市场新低状态；全局标定仍按 Forex 只降不升
- 市场新低状态持久化到 `alert_states`，重启后恢复
- Forex 参考价超过 `forex_max_age_hours` 后不再参与告警计算
- 删除持久化市场新低失败时，内存状态保持不变，避免重启后状态反弹

### 服务状态

- `GET /healthz` 只表示 HTTP 进程存活
- `GET /readyz` 要求当前存在未过期的 Forex 参考价
- `GET /api/status` 按交易所返回：
  - `OK`：所有金额档位都返回数据
  - `Degraded`：至少一个金额档位成功，但存在失败或空结果
  - `Error`：本轮没有任何金额档位返回数据
- 上游 Forex 拉取失败但数据库缓存仍在有效期内时，服务可继续读取和展示数据；Forex 状态仍保留上游错误信息
- Forex 不可用或过期时仍继续采集 C2C 历史价格，但暂停价差机会告警

### 运行时配置

- `GET /api/config` 返回当前监控配置
- `GET /api/meta` 返回当前服务版本以及前端渲染所需的交易所元数据
- `GET /api/changelog` 返回版本变更记录
- `POST /api/config` 更新运行中配置，需要 `Authorization: Bearer <admin_token>`
- `GET /api/alerts/benchmark` 返回全局默认标定；增加 `?amount=<target_amount>` 后返回对应档位的有效标定
- `POST /api/alerts/benchmark` 持久化一个更低的默认或档位标定价，需要管理员 Bearer token
- `POST /api/alerts/reset` 清除指定市场的最近告警价格，使其重新使用对应档位标定，同样需要管理员 Bearer token
- `POST /api/config` 只影响内存态且不回写 `config.yaml`；告警标定价单独持久化到数据库
- 前端只把管理员 token 保存在当前标签页的 `sessionStorage`，关闭标签页后自动清除

### 安全边界

- `app.admin_token` 启动时必须显式配置，长度至少 16 个字符
- 跨域 API 请求只允许 `app.allowed_origins` 中列出的来源
- 生产 K3s 前端使用同源 Nginx 代理，默认不需要开启 CORS
- 管理写接口限制请求体大小；HTTP 服务、交易所请求和 SMTP 发送都有明确超时
- 没有 SMTP 凭据的部署必须显式关闭邮件通知，不能用持续失败的伪配置冒充可用通知

## 明确非目标

- 不支持前端直接修改持久化配置文件
- 不做自动交易或自动下单
- 不做多币种、多法币、多方向的一般化抽象
- 不提供多用户、角色或细粒度权限系统；当前只有一个部署级管理员 token

## 对后续迭代的要求

- 如果新增交易所，先更新交易所单一事实来源和文档，再接入适配器
- 如果改动告警策略，必须同步更新本文件与相关测试
- 如果新增更复杂的后台任务，优先在 `docs/exec-plans/active/` 写执行计划
