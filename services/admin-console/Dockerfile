FROM golang:1.24-bookworm AS builder

WORKDIR /build

COPY go.mod go.sum* ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY web ./web

RUN CGO_ENABLED=0 GOOS=linux GOFLAGS='-p=1' go build -o admin-console ./cmd/admin-console

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /build/admin-console ./admin-console
COPY --from=builder /build/web ./web

EXPOSE 9340
CMD ["./admin-console"]
