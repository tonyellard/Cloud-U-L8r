# SPDX-License-Identifier: Apache-2.0
# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Copy shared packages
COPY pkg/ ./pkg/

# Copy service module files and download deps
COPY services/pipes/go.mod ./services/pipes/
WORKDIR /build/services/pipes
RUN go mod download

# Copy service source code
WORKDIR /build
COPY services/pipes/ ./services/pipes/

# Build binary
WORKDIR /build/services/pipes
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o pipes ./cmd/pipes

# Runtime stage
FROM alpine:3.21

WORKDIR /app

RUN apk --no-cache add ca-certificates

COPY --from=builder /build/services/pipes/pipes ./pipes

EXPOSE 9344

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost:9344/health || exit 1

ENTRYPOINT ["./pipes"]
