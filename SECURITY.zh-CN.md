# 安全策略

English version: [SECURITY.md](SECURITY.md)

欢迎报告安全问题。漏洞应先通过私有渠道处理，再公开披露。

## 支持版本

在稳定 release branches 发布前，UModel Open Source 的安全修复优先进入主开发线。存在版本化 release branches 后，应按需要 backport。

| Version | Supported |
|---|---:|
| `main` | Yes |
| Tagged releases | Not yet published |

## 报告漏洞

请不要为漏洞创建公开 issue。

优先路径：

1. 如果仓库启用了 GitHub private vulnerability reporting，请使用该功能。
2. 如果不可用，请通过托管组织列出的维护者私有渠道联系。

请包含：

- 受影响 commit、branch 或 release。
- 复现步骤。
- 期望行为和实际行为。
- 影响评估。
- 已知 workaround。

## 维护者响应

维护者应在收到完整报告后 5 个工作日内确认，评估严重性，并与报告者协商修复或披露计划。

## 安全边界

当前开源安全默认值：

- `make dev`、Docker 和 Compose 使用 `file.memory` 本地持久化。
- MCP 写工具默认关闭。
- AgentGateway resources 暴露元数据和模板，不暴露运行时 rows。
- UModel 导入端点（`POST /api/v1/umodel/{workspace}/import`、`umctl umodel import`）按设计从 operator 提供的**服务端本地文件路径**读取模型包，类似 `kubectl apply -f <path>`。这是 operator 能力，不得暴露给不受信任的调用方。请勿将导入端点放在公网可达的表面，或在其前面加认证/授权。
- 当前 release 不包含 multi-tenant authorization 或 cloud-hosted control plane 行为。

不要在没有认证、授权、传输安全、限流、审计和部署加固的情况下，把本地开发服务作为公网生产服务使用。
