# entity_set_link

EntitySetLink 用于定义两个 EntitySet 之间的关系。EntitySetLink 必须包含源 EntitySet 、目标 EntitySet 和链接类型。

**Kind**: `entity_set_link`

> 每个元素共享标准信封 `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

**继承**: [link](../../shared-types#link)

## `spec` 字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `fields_mapping` | map&lt;string, string&gt; |  |  | EntitySetLink 的目标 EntitySet 字段映射。 |
| `constructor` | object |  |  | EntitySetLink 的自动化生成配置。若配置了此字段，则后台会自动创建并维护任务，自动生成 EntitySetLink。 |
| `entity_link_type` | `string` | 是 |  | EntityLinkSet 的链接类型。值不能为空。建议的取值有： - calls (调用) - runs (运行) - instance_of (实例) - parent_of (父级) - contains (包含) - balances (平衡) - can_access (可访问) - clustered_by (集群化) - manages (管理) - monitors (监控) - sends_to (发送到) -… |
| `dynamic` | `bool` |  | `false` | 是否为动态生成，默认为false。为true时表示该 EntitySetLink 为动态 Link，会绑定 Storage，每次 Link 的内容由Storage 处获取，而非存储在 EntityStore 中。 |
