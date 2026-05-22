# Web UI API 对照

English: [UModel Web UI API Map](../en/ui-api.md)

Web UI 的每个界面只对应公开 REST 契约。

| 界面 | 操作 | REST API | 请求体 |
|---|---|---|---|
| Workspace 选择页 | 健康检查 | `GET /healthz` | 无 |
| Workspace 选择页 | 列出 Workspace | `GET /api/v1/workspaces?page_size=100&include_conflicts=true` | 无 |
| Workspace 选择页 | 创建 Workspace | `POST /api/v1/workspaces` | `CreateWorkspaceRequest` |
| Workspace 工作台 | 加载当前 Workspace | `GET /api/v1/workspaces/{workspace}` | 无 |
| Settings | 更新 Workspace | `PUT /api/v1/workspaces/{workspace}` | `UpdateWorkspaceRequest` |
| Settings | 删除 Workspace | `DELETE /api/v1/workspaces/{workspace}` | 无 |
| Explorer | 加载模型图/表 | `POST /api/v1/query/{workspace}/execute` | `{"query": ".umodel | sort id | limit 100", "limit": 100}` |
| Explorer 详情 | 校验元素 JSON | `POST /api/v1/umodel/{workspace}/validate` | `{"elements": [UModelElement]}` |
| Explorer 详情 | 保存元素 JSON | `POST /api/v1/umodel/{workspace}/elements` | `{"elements": [UModelElement]}` |
| Explorer 详情 | 删除元素 | `DELETE /api/v1/umodel/{workspace}/elements` | `{"ids": ["element-id"]}` |
| Query 控制台 | 执行 SPL | `POST /api/v1/query/{workspace}/execute` | `QueryRequest` |
| Query 控制台 | Explain SPL | `POST /api/v1/query/{workspace}/explain` | `QueryRequest` |
| Data Store | 实体查询 | `POST /api/v1/query/{workspace}/execute` | `.entity` `QueryRequest` |
| Data Store | 拓扑查询 | `POST /api/v1/query/{workspace}/execute` | `.topo` `QueryRequest` |
| Imports | 导入服务端路径 | `POST /api/v1/umodel/{workspace}/import` | `UModelImportRequest` |
| Imports | 导入内置 quickstart 样例 | `POST /api/v1/samples/{workspace}/multi-domain-quickstart:import` | 无 |
| Imports | 校验 UModel JSON | `POST /api/v1/umodel/{workspace}/validate` | `{"elements": [...]}` |
| Imports | 写入 UModel JSON | `POST /api/v1/umodel/{workspace}/elements` | `{"elements": [...]}` |
| Imports | 写入实体 | `POST /api/v1/entitystore/{workspace}/entities:write` | `EntityWriteBatch` |
| Imports | 写入关系 | `POST /api/v1/entitystore/{workspace}/relations:write` | `RelationWriteBatch` |
| Imports | 过期实体 | `POST /api/v1/entitystore/{workspace}/entities:expire` | `ExpireRequest` |
| Imports | 过期关系 | `POST /api/v1/entitystore/{workspace}/relations:expire` | `ExpireRequest` |
| Agent | Discovery | `GET /api/v1/agent/{workspace}/discover` | 无 |
| Agent | 读取 Resource | `POST /api/v1/agent/{workspace}/resources:read` | `{"uri": "umodel://..."}` |
| Agent | 执行 Tool | `POST /api/v1/agent/{workspace}/tools:execute` | `AgentToolCallRequest` |

## 边界

- UI 只使用公开 REST API。
- UI 不需要 cloud tenant、account、region 或 Aliyun-specific frontend dependency。
- Graph/table model view 从 Query Service rows 派生，而不是来自 private snapshot endpoint。
- 开源 UI 中 UModel forms 为 JSON-only。未来的 schema-aware editors 继续复用同一组 `validate` 和 `elements` APIs。
