FROM golang:1.22 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY pkg ./pkg
RUN CGO_ENABLED=0 go build -o /out/umodel-server ./cmd/umodel-server

FROM node:22-bookworm AS ui
WORKDIR /web
RUN corepack enable && corepack prepare pnpm@9.15.9 --activate
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

# debian-slim keeps a shell for kubectl/docker exec debugging.
# distroless has no /bin/sh and blocks interactive troubleshooting.
FROM debian:bookworm-slim
RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates \
  && rm -rf /var/lib/apt/lists/* \
  && mkdir -p /data /app/ui
WORKDIR /app
COPY --from=build /out/umodel-server /usr/local/bin/umodel-server
COPY --from=ui /web/dist /app/ui
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/umodel-server"]
CMD ["--addr", ":8080", "--data", "/data", "--graphstore", "file.memory", "--ui-dir", "/app/ui"]
