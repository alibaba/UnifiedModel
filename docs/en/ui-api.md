# UModel Web UI API Map

中文：[Web UI API 对照](../zh/ui-api.md)

The web UI maps each screen to the public REST contract below.

| UI screen | Operation | REST API | Request body |
|---|---|---|---|
| Workspace chooser | Health | `GET /healthz` | none |
| Workspace chooser | List workspaces | `GET /api/v1/workspaces?page_size=100&include_conflicts=true` | none |
| Workspace chooser | Create workspace | `POST /api/v1/workspaces` | `CreateWorkspaceRequest` |
| Workspace shell | Load selected workspace | `GET /api/v1/workspaces/{workspace}` | none |
| Settings | Update workspace | `PUT /api/v1/workspaces/{workspace}` | `UpdateWorkspaceRequest` |
| Settings | Delete workspace | `DELETE /api/v1/workspaces/{workspace}` | none |
| Explorer | Load model graph/table | `POST /api/v1/query/{workspace}/execute` | `{"query": ".umodel | sort id | limit 100", "limit": 100}` |
| Explorer detail | Validate element JSON | `POST /api/v1/umodel/{workspace}/validate` | `{"elements": [UModelElement]}` |
| Explorer detail | Save element JSON | `POST /api/v1/umodel/{workspace}/elements` | `{"elements": [UModelElement]}` |
| Explorer detail | Delete element | `DELETE /api/v1/umodel/{workspace}/elements` | `{"ids": ["element-id"]}` |
| Query console | Execute SPL | `POST /api/v1/query/{workspace}/execute` | `QueryRequest` |
| Query console | Explain SPL | `POST /api/v1/query/{workspace}/explain` | `QueryRequest` |
| Data Store | Entity search | `POST /api/v1/query/{workspace}/execute` | `.entity ...` `QueryRequest` |
| Data Store | Topology query | `POST /api/v1/query/{workspace}/execute` | `.topo ...` `QueryRequest` |
| Imports | Import server path | `POST /api/v1/umodel/{workspace}/import` | `UModelImportRequest` |
| Imports | Import bundled quickstart sample | `POST /api/v1/samples/{workspace}/multi-domain-quickstart:import` | none |
| Imports | Validate UModel JSON | `POST /api/v1/umodel/{workspace}/validate` | `{"elements": [...]}` |
| Imports | Put UModel JSON | `POST /api/v1/umodel/{workspace}/elements` | `{"elements": [...]}` |
| Imports | Write entities | `POST /api/v1/entitystore/{workspace}/entities:write` | `EntityWriteBatch` |
| Imports | Write relations | `POST /api/v1/entitystore/{workspace}/relations:write` | `RelationWriteBatch` |
| Imports | Expire entities | `POST /api/v1/entitystore/{workspace}/entities:expire` | `ExpireRequest` |
| Imports | Expire relations | `POST /api/v1/entitystore/{workspace}/relations:expire` | `ExpireRequest` |
| Agent | Discover | `GET /api/v1/agent/{workspace}/discover` | none |
| Agent | Read resource | `POST /api/v1/agent/{workspace}/resources:read` | `{"uri": "umodel://..."}` |
| Agent | Execute tool | `POST /api/v1/agent/{workspace}/tools:execute` | `AgentToolCallRequest` |

## API Boundary

- The UI does not call server internals.
- The UI does not need a cloud tenant, account, region, or Aliyun-specific frontend dependency.
- The graph/table model view is derived from Query Service rows rather than a private snapshot endpoint.
- UModel forms are JSON-only in the open-source UI. Future schema-aware editors can be added behind the same `validate` and `elements` APIs.
