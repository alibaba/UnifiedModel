# 使用稳定、按域限定的身份

用稳定的、按 domain 限定的字段来标识实体——不要用显示名或易变属性。

## 为什么

实体 ID 由其 `primary_key_fields` 经 entity set 的 `id_generator` 生成。如果身份依赖显示名或会变的值（标签、可变状态），同一个现实对象在不同时间会算出不同 ID——破坏拓扑边、去重和历史连续性。稳定身份让一个实体在多次观测、文档、测试、截图中始终是*同一个*实体。

## 该做 / 不该做

| 该做 | 不该做 |
|---|---|
| 选域内不可变且唯一的 `primary_key_fields`（如资源 ARN，或 `cluster` + `namespace` + `name` 组合）。 | 用 `display_name` 或面向人的标签作身份。 |
| 保持 entity set 的 `name` 稳定、按域限定（`devops.service`）。 | 改名来改变含义，或跨域复用同一个名字。 |
| 在样例数据、测试、文档间复用同一批 ID。 | 每个环境重新生成 ID。 |

## 参见

- [模型编写](/zh/guides/model-authoring)
- [EntitySet 实体集](/zh/concepts/entity-sets)
- [实体与关系写入](/zh/guides/entity-relation-writes)
