# UModel server with the example packs baked in, for the quickstart demo stack.
# Build context is the repo root (see docker-compose.yml).
FROM golang:1.22 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY pkg ./pkg
RUN CGO_ENABLED=0 go build -o /out/umodel-server ./cmd/umodel-server

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=build /out/umodel-server /usr/local/bin/umodel-server
# Sample packs are read from disk at runtime, so ship examples/ in the image.
COPY examples /app/examples
EXPOSE 8080
ENTRYPOINT ["umodel-server"]
CMD ["--quickstart", "--quickstart-sample", "multi-domain-quickstart", "--graphstore", "memory", "--addr", ":8080"]
