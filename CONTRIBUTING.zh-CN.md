# UModel 贡献指南

English version: [CONTRIBUTING.md](CONTRIBUTING.md)

欢迎参与 UModel 共建。本仓库是 UModel 对象图语义层的开源实现，贡献应保持公共 API、CLI、MCP、SDK、示例和文档一致。

## 环境要求

| 工具 | 最低版本 | 用途 |
|---|---:|---|
| Go | 1.22 | 服务端、CLI、MCP、测试、Go SDK 检查 |
| Python | 3.10 | Schema 展开、校验、代码生成 |
| Make | 近期版本 | 本地工作流 |
| Node.js | 22 | Web UI |
| pnpm | 首选 9 | Web UI 依赖；支持 corepack 或 npm exec fallback |
| Java + Maven | Java 8 / Maven 3.6 | Java SDK 校验 |

检查并安装依赖：

```bash
make check-env
make install-env
```

`make check-env` 会检查本地工具链。`make install-env` 会创建 `.venv`、安装 Python 依赖、下载 Go modules、安装 Web UI 依赖，并在 Maven 可用时预解析 Java 依赖。`make setup` 保留为 `make install-env` 的别名。

## 常用工作流

构建服务：

```bash
make build-service
```

启动本地 API 和 Web UI：

```bash
make dev
```

停止服务：

```bash
make stop-all
```

运行服务端测试：

```bash
make test-service
```

运行架构与公开链接检查：

```bash
make guard
```

校验示例：

```bash
make example-validate
```

验证生成 SDK：

```bash
make verify
```

`make build-ui`、`make test-ui` 和 `make dev-web` 都使用 Makefile 环境 wrapper；当 `corepack`、`npm exec` 或已有 `web/node_modules` 能满足工作流时，贡献者不需要直接调用 `pnpm`。

## 贡献原则

- 公共读取统一通过 Query Service，不新增分散的 entity/relation/topology 读取 API。
- 公共契约变化时，同步更新 OpenAPI、MCP schema、CLI、SDK、Web UI、测试和文档。
- 模型、示例数据和文档要在同一个 PR 中保持一致。
- 新增文档或实质修改文档时，同时更新英文和中文版本。
- 业务模块不得直接依赖 GraphStore provider 实现包。
- Schema 描述和公开文档应包含完整说明。先从源 Schema 中移除内部钉钉文档引用，再生成 SDK 和文档。`make public-links-check` 检查源 Schema、展开与嵌入 Schema、SDK 和公开文档；`make guard` 与 `make ci` 会执行该检查，发现内部文档链接时失败。

## 文档

文档入口：

- 双语文档总入口：[docs/README.md](docs/README.md)
- 中文导航：[docs/zh/README.md](docs/zh/README.md)

新增文档时请遵循 [docs/README.md](docs/README.md) 中的双语维护规则。

## Pull Request 检查清单

- 描述清楚用户场景和公共接口影响。
- 说明运行过的测试和验证命令。
- 如果修改了契约，同步更新相关客户端和文档。
- 如果修改了示例，同步更新 example README 和 query 示例。
- 如果修改了文档，确认中英文版本都存在且链接有效。
