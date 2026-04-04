# Docs Index

`docs/` 是这个仓库的记录系统。顶层 `AGENTS.md` 只负责导航，这里负责放“真实可维护的上下文”。

## 阅读顺序

1. [`../AGENTS.md`](../AGENTS.md)
2. [`product/monitoring.md`](product/monitoring.md)
3. [`architecture/overview.md`](architecture/overview.md)
4. [`operations/runbook.md`](operations/runbook.md)
5. [`exec-plans/active/2026-04-harness-foundation.md`](exec-plans/active/2026-04-harness-foundation.md)

## 文档职责

- `product/`：产品目标、范围、行为约束、已知非目标
- `architecture/`：模块边界、依赖方向、关键实现约束
- `operations/`：启动、验证、排障、运行手册
- `exec-plans/active/`：仍在推进中的计划、决策、检查点
- `exec-plans/completed/`：已完成计划的归档位
- `tech-debt-tracker.md`：确认过、但尚未投入解决的工程债务

## 更新规则

- 文档要替换陈旧信息，不要叠加相互矛盾的描述。
- 如果代码和文档冲突，以“修文档或修代码其一”作为本次变更的一部分，不把漂移留到以后。
- 新增大范围改动前，先写计划，再动代码。
