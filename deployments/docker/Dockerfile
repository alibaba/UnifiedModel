FROM golang:1.22 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY pkg ./pkg
RUN go build -o /out/umodel-server ./cmd/umodel-server

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=build /out/umodel-server /usr/local/bin/umodel-server
EXPOSE 8080
ENTRYPOINT ["umodel-server"]
CMD ["--addr", ":8080", "--data", "/data", "--graphstore", "file.memory"]
