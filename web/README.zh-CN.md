# UModel Web UI

English version: [README.md](README.md)

UModel Web 是面向公开 UModel REST API 的 React/Vite workspace UI。

## 开发

要求：

- Node.js 22 或更新版本。
- pnpm 9 或更新版本。

启动 API：

```bash
go run ./cmd/umodel-server --addr :8080 --data data --graphstore file.memory
```

启动 Web dev server：

```bash
cd web
pnpm install
pnpm dev
```

Vite server 会将 `/api/*` 和 `/healthz` 代理到 `http://localhost:8080`。可以通过 `UMODEL_API_TARGET` 指向其他 API server。

## 生产构建

```bash
cd web
pnpm build
```

Go server 可以托管生成资产：

```bash
go run ./cmd/umodel-server --addr :8080 --data data --graphstore file.memory --ui-dir web/dist
```

仓库根目录也可以使用：

```bash
make dev
make stop-all
make serve-ui
```

`make dev` 默认使用 `file.memory` 和 `DATA_ROOT=data`，因此通过 UI 添加的数据在 API 重启后仍会保留。
