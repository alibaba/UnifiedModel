# UModel server with the example packs baked in, for the incident-investigation demo stack.
# Build context is the repo root (see docker-compose.yml).
FROM golang:1.22 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY pkg ./pkg
RUN CGO_ENABLED=0 go build -o /out/umodel-server ./cmd/umodel-server

# python base (not distroless) so the entrypoint can slide the sample timeline to "now"
# before the server starts. The server itself is a static CGO_ENABLED=0 binary.
FROM python:3.12-slim
WORKDIR /app
COPY --from=build /out/umodel-server /usr/local/bin/umodel-server
# Sample packs are read from disk at runtime, so ship examples/ in the image.
COPY examples /app/examples
# entrypoint.py re-anchors the sample timeline (deploy/config/incident/promotion timestamps)
# to be relative to the current time, so the demo never expires, then execs the server.
COPY examples/incident-investigation/deploy/entrypoint.py /app/entrypoint.py
EXPOSE 8080
ENTRYPOINT ["python", "/app/entrypoint.py"]
CMD ["--quickstart", "--quickstart-sample", "incident-investigation", "--graphstore", "memory", "--addr", ":8080"]
