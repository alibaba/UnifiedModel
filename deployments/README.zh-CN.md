# 部署

English version: [README.md](README.md)

本目录包含 UModel Open Source 的本地部署资产。

| 路径 | 作用 |
|---|---|
| `docker/Dockerfile` | 将 `umodel-server` 构建为小型运行时镜像。 |
| `compose/docker-compose.yaml` | 使用持久化 Docker volume 运行服务。 |
| `compose/docker-compose.neo4j.yaml` | 同时运行 Neo4j 和 UModel，分别持久化图数据和工作区元数据。 |

## 默认 Provider

开源部署资产默认使用 `--graphstore file.memory`。这样不需要本地 Ladybug runtime，并将 GraphStore JSON 数据持久化到 `/data`。

仅在具备 Ladybug-enabled build、`liblbug` runtime 且有明确运维原因时使用 `local.ladybug`。

## Docker

```bash
docker build -f deployments/docker/Dockerfile -t umodel-open-source:local .
docker run --rm \
  -p 8080:8080 \
  -v umodel-data:/data \
  umodel-open-source:local
```

健康检查：

```bash
curl http://localhost:8080/healthz
```

## Docker Compose

```bash
docker compose -f deployments/compose/docker-compose.yaml up --build
docker compose -f deployments/compose/docker-compose.yaml down
```

删除持久化数据：

```bash
docker compose -f deployments/compose/docker-compose.yaml down -v
```

## Neo4j Compose

```bash
export NEO4J_PASSWORD='replace-with-your-password'
docker compose -f deployments/compose/docker-compose.neo4j.yaml up --build
```

该配置选择 `--graphstore neo4j`，等待 Neo4j 健康后再启动 UModel。Neo4j Browser 和 Bolt 使用本地端口 7474、7687，UModel 使用 8080。图数据使用 `neo4j-data` volume，工作区元数据使用 `umodel-neo4j-metadata` volume，重启时应保留两者。

连接已有数据库时，将 `NEO4J_URI`、`NEO4J_USERNAME`、`NEO4J_PASSWORD`、`NEO4J_DATABASE` 传入容器，并将启动参数设为 `--addr :8080 --data /data --graphstore neo4j`。配置与查询限制见 [GraphStore Providers](../docs/zh/graphstore-providers.md#neo4j)。

## 端口与数据

| 配置 | 默认值 | 说明 |
|---|---|---|
| API port | `8080` | `http://localhost:8080` |
| Data directory | `/data` | Compose 中挂载到 `umodel-data` volume。 |
| GraphStore provider | `file.memory` | JSON snapshot 位于 `/data/graphstore/file-memory/`。 |

## 导入 Demo

```bash
go run ./cmd/umctl --addr http://localhost:8080 workspace create demo '{"name":"Demo"}'
curl -X POST http://localhost:8080/api/v1/samples/demo/multi-domain-quickstart:import \
  -H 'Content-Type: application/json' \
  -d '{}'
go run ./cmd/umctl --addr http://localhost:8080 query run demo ".umodel | limit 5"
```
