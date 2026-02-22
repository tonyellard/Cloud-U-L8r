# SPDX-License-Identifier: Apache-2.0
# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Copy shared packages
COPY pkg/ ./pkg/

# Copy service module files and download deps
COPY services/scheduler/go.mod services/scheduler/go.sum ./services/scheduler/
WORKDIR /build/services/scheduler
RUN go mod download

# Copy service source code
WORKDIR /build
COPY services/scheduler/ ./services/scheduler/

# Build binary
WORKDIR /build/services/scheduler
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o scheduler ./cmd/scheduler

# Runtime stage
FROM alpine:3.21

WORKDIR /app

RUN apk --no-cache add ca-certificates

COPY --from=builder /build/services/scheduler/scheduler ./scheduler

EXPOSE 9360

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost:9360/health || exit 1

ENTRYPOINT ["./scheduler"]
