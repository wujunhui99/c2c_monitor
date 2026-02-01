这是一个标准的 产品需求文档 (PRD)，你可以直接依据此文档进行开发。
项目名称：C2C 溢价/折价 监测系统 (C2C-Arb-Monitor)
版本: v1.0.0
状态: 开发中
1. 项目背景与目标
  CEX (如 Binance, Gate) 的 C2C 市场中，USDT 对 CNY 的价格受供需关系影响，经常与国际外汇市场 (USD-CNY) 存在价差。
  目标：构建一个自动化的监测系统，捕捉 USDT 价格低于美元汇率（折价）的时机，通过邮件报警提示用户买入，并提供历史数据趋势图以便分析。
  核心原则：工程化、高内聚低耦合、依赖倒置（面向接口编程）。
2. 技术栈选型 (Technology Stack)
  基于“工程化”与“依赖倒置”的要求，选型如下：
  编程语言: Golang (Go 1.21+)
  理由: 强类型接口支持依赖倒置；原生并发处理多交易所轮询；部署简单（单二进制文件）。
  Web 框架: Gin理由: 轻量级、高性能，用于提供 API 给前端可视化。
  数据库: MySQL理由: 使用最广泛，生态良好。
  ORM (可选): GORM理由: 加速 CRUD 开发，屏蔽 SQL 细节（如需极致性能可换 sqlx，但本项目 GORM 足够）。
  可视化: ECharts (前端) + 原生 HTML/JS
  理由: 百度开源图表库，功能强大。不引入 React/Vue 构建流程，保持项目结构简单清晰。
  配置管理: Viper理由: 支持 YAML 配置文件读取，热重载。
3. 详细功能需求 (Functional Requirements)
  3.1 数据采集 (Data Collection)
  C2C 市场价格采集
  此模块负责高频监听各大交易所的 P2P 市场价格。
  基础配置:
  支持交易所: Binance, Gate.io, OKX (采用 Adapter 模式)。
  交易对: USDT/CNY。
  交易方向:
  Buy (默认): 监测用户买入 USDT 的价格（即商家广告单的卖出价）。
  Sell (预留): 监测用户卖出 USDT 的价格（即商家广告单的买入价），用于未来计算搬砖价差。
  采集频率: 默认 3分钟 (可配置 c2c_interval)。
  多金额档位监测 (核心逻辑)，系统需支持配置一个金额列表（默认[30, 50, 200, 500, 1000] CNY）。
  此外，还需支持 **最低价格 (Lowest Price)** 监测，即不限制金额档位，获取市场上的绝对最优价 (对应配置 amount=0)。
  取价算法:
  只获取最低价格。
  数据存储: 始终存储采集到的最低价格数据（即 Rank 1 的价格记录），不进行平均值预计算，以便保留完整市场深度数据供后续分析。
  B. 外汇汇率采集
  数据源: Yahoo Finance API (首选，免费且稳定) 或 ExchangeRate-API。
  交易对: USD/CNY (离岸人民币或在岸人民币，建议统一使用 USDCNY=X 即标准汇率)。
  采集频率: 默认 1小时 (可配置 forex_interval)。
  3.2 报警系统 (Alerting)
  触发条件:
  监测逻辑: (外汇汇率 - C2C价格) / 外汇汇率 >= 阈值。
  示例: 汇率 7.20，阈值 0.1%。如果 USDT 价格 < 7.164，则触发报警。
  报警方式:
  Email: 发送纯文本或 HTML 邮件。
  内容: 包含当前 USDT 价格、外汇汇率、价差百分比、触发时间的交易所。
  防抖动:
  同一交易所触发报警后，N 分钟内不再重复发送相同类型的报警（推荐冷却时间：30分钟）。
  3.3 数据存储 (Storage)
  Schema 设计:
  C2C 价格表 (c2c_prices)
   id (BIGINT, PK): 自增主键。
   created_at (DATETIME): 数据采集时间（建议精确到毫秒，便于高频排序）。
   exchange (VARCHAR): 交易所标识 (e.g., "Binance", "Gate")。
   symbol (VARCHAR): 数字货币符号 (e.g., "USDT")。
   fiat (VARCHAR): 法币符号 (e.g., "CNY")。
   side (VARCHAR): 交易方向 ("BUY" | "SELL")。
   target_amount (INT): 关键字段。表示该价格对应的交易金额档位 (e.g., 30, 200, 500, 1000)。0 代表无金额限制（最低价）。
   rank (int): 当前价格排名，例如 1 代表排名最低的价格, 2 代表排名倒数第二低的价格, 3 代表排名倒数第三低的价格
   price (DECIMAL(18, 8)): 价格。
   Indexes:
   复合索引 idx_query: (exchange, side, target_amount, rank, created_at) —— 优化最常用的查询场景：“查询某交易所、某方向、某金额档位、第rank 优报价, 在特定时间段内的价格走势”。
  外汇汇率表 (forex_rates)
   id (BIGINT, PK): 自增主键。
   created_at (DATETIME): 数据采集时间。
   source (VARCHAR): 数据来源 (e.g., "Yahoo", "ExchangeRateAPI")。
   pair (VARCHAR): 货币对 (e.g., "USDCNY")。
   rate (DECIMAL(18, 6)): 汇率值。
   Indexes:
   时间索引 idx_time: (created_at) —— 用于快速获取最新的外汇汇率（ORDER BY created_at DESC LIMIT 1）
  3.4 可视化展示 (Visualization)

    ```markdown
    A. 界面布局 (Layout)
    架构: 单页应用 (SPA)。
    分区: 界面分为上下两部分（或通过 Tab 切换）：
    数据看板 (Dashboard): 展示历史趋势图。
    系统设置 (Settings): 调节采集参数与金额档位。
    B. 数据看板 (Dashboard)
    用于分析历史价格走势与套利空间。
    
    全局筛选栏 (Control Bar):
    
    金额档位选择器 (Amount Selector): [下拉框]
    逻辑: 读取当前配置的 target_amount 列表 (e.g., 0, 30, 500...)。0 显示为 "最低价 (Lowest)"。
    作用: 切换图表数据源。例如选择 "500 CNY"，图表即渲染各大交易所在 500 元额度下的买入价走势。
    时间过滤器 (Time Range):
    按钮组：[24小时], [7天], [30天], [全部]。
    核心图表 (Main Chart):
    
    库: ECharts。
    X轴: 时间 (Time)。
    Y轴: 价格 (CNY) - 自动缩放坐标轴范围以突显微小价差。
    数据序列 (Series):
    Line 1 (Binance): 对应选定金额档位的 Binance USDT 价格。
    Line 2 (Gate): 对应选定金额档位的 Gate USDT 价格。
    Line 3 (Forex Reference): USD/CNY 国际汇率（基准线，颜色高亮/虚线区分）。
    交互: 鼠标悬停显示具体时刻的价差百分比 (Spread %)。
    C. 系统设置 (Settings Panel)
    用于动态调整后端采集策略，无需重启服务。
    
    采集频率配置:
    
    C2C 轮询间隔: [输入框] (单位: 分钟)。默认: 6。
    外汇 轮询间隔: [输入框] (单位: 小时)。默认: 1。
    金额档位管理 (Target Amount Manager):
    
    展示形式: 标签列表 (Tag List)。
    操作:
    新增: 输入数字 (e.g., 500) -> 点击 [添加]。后端下一次轮询即开始采集 500 USD 档位的数据。
    删除: 点击标签旁的 [x] -> 确认删除。后端停止采集该档位（历史数据保留在数据库中，但不显示在图表选项里）。
    操作反馈:
    
    保存按钮: 点击后发送 API 请求更新服务端内存配置及配置文件。
    状态提示: 显示“配置已更新，下一次轮询生效”。(注：前端需处理新增档位在首次轮询前“暂无数据”的显示状态)
  
4. 模块设计与架构 (Architecture Design)
  采用 六边形架构 (Hexagonal Architecture) 思想。
  4.1 核心层 (Internal/Domain)
  定义纯粹的接口和实体，不含任何 tag (json/sql) 以外的外部依赖。
  Entities: PricePoint(价格点), AlertRule(报警规则，包含差价阈值，冷却时间等).
  Interfaces:
  IExchange: GetPrice(fiat, coin, side) (float64, error)
  IForex: GetRate(from, to) (float64, error)
  INotifier: Send(subject, body) error
  IRepository: Save(PricePoint), Query(start, end) []PricePoint
  4.2 适配器层 (Internal/Infrastructure)
  实现核心层的接口。
  BinanceAdapter: 实现 IExchange。调用 Binance P2P API。
  GateAdapter: 实现 IExchange。调用 Gate P2P API。
  OKXAdapter: 实现 IExchange。调用 OKX P2P API。
  YahooForexAdapter: 实现 IForex。
  EmailAdapter: 实现 INotifier (基于 net/smtp)。
  MySQLRepo: 实现 IRepository (基于 GORM)。
  4.3 应用层 (Internal/Service)
  负责业务逻辑编排。
  MonitorService:
  持有 []IExchange, IForex, INotifier, IRepository。
  启动两个 Goroutine (Ticker)：
  Loop 1 (3min): 遍历 Exchanges -> 获取价格 -> 存库 -> 对比 cached Forex -> 报警。
  Loop 2 (1h): 获取 Forex -> 存库 -> 更新 cached Forex。
  4.4 接入层 (Cmd & API)
  main.go: 负责依赖注入 (Dependency Injection)，读取 Config，组装 Service。
  api/handler.go: 处理 Gin 的 HTTP 请求，调用 Repo 查询数据返回 JSON。
5. API 接口定义
  GET /api/v1/history
  Query Params:
  range: 1d | 7d | 30d | all
  amount: int (required, e.g., 100)
  rank: int (optional, default=1)
  Response:
  {
    "code": 200,
    "data": {
   "forex": [{"t": 1700000000, "v": 7.21}, ...],
   "binance": [{"t": 1700000000, "v": 7.15}, ...],
   "gate": [{"t": 1700000000, "v": 7.16}, ...]
    }
  }

  `GET /api/config`: 获取当前配置（包含当前的金额档位列表、时间间隔）。

  `POST /api/config`: 更新配置。

  - Request Body:

    ```json
    {
      "c2c_interval_minutes": 5,
      "forex_interval_hours": 1,
      "target_amounts": [0, 30, 50, 200, 500, 1000]
    }
    ```

6. 配置文件 (config.yaml)
  app:
    port: 8080

monitor:
  c2c_interval_minutes: 3
  forex_interval_hours: 1
  alert_threshold_percent: 0.1 # 0.1% difference

notification:
  email:
   ....

database:
  ...

binance
  ...

gate
  ...

7. 开发计划 (Milestones)
Phase 1: 核心与基础设施 (Core & Infra)
定义 Domain 接口。
实现 MySQL 存储。
实现 Email 通知模块。
Phase 2: 数据采集实现 (Adapters)实现 Yahoo Forex 采集。
实现 Binance/Gate P2P API (使用官方api)
Phase 3: 业务逻辑 (Service)编写调度器，串联采集与报警。
实现 (Forex - C2C) / Forex 报警逻辑。
Phase 4: 可视化 (Web)搭建 Gin Server。
编写 HTML + ECharts 页面。
联调数据展示。

