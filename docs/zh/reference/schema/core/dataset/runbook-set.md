# runbook_set

RunbookSet 用于定义实体、数据的操作手册集合，包含分析工具、操作指南和最佳实践。

**Kind**: `runbook_set`

> 每个元素共享标准信封 `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

## `spec` 字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `observations` | array&lt;[observation](../../shared-types#observation)&gt; |  |  | 观察配置列表 |
| `actions` | array&lt;object&gt; |  |  | 操作能力配置列表 |
| `toolkits` | array&lt;object&gt; |  |  | 工具箱配置列表，每个工具箱包含共享配置和一组工具 |
| `knowledge` | array&lt;object&gt; |  |  | 知识库配置列表 |
| `automations` | array&lt;object&gt; |  |  | 自动化配置列表 |
| `skills` | array&lt;object&gt; |  |  | 技能配置列表，遵循 Agent Skills 规范 |
