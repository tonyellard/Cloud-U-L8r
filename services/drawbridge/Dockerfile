# SPDX-License-Identifier: Apache-2.0
# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Copy shared packages
COPY pkg/ ./pkg/

# Copy service module files and download deps
COPY services/drawbridge/go.mod services/drawbridge/go.sum ./services/drawbridge/
WORKDIR /build/services/drawbridge
RUN go mod download

# Copy service source code
WORKDIR /build
COPY services/drawbridge/ ./services/drawbridge/

# Build binary
WORKDIR /build/services/drawbridge
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o drawbridge ./cmd/drawbridge

# Runtime stage
FROM alpine:3.21

WORKDIR /app

RUN apk --no-cache add ca-certificates

COPY --from=builder /build/services/drawbridge/drawbridge ./drawbridge

EXPOSE 9340

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost:9340/health || exit 1

ENTRYPOINT ["./drawbridge"]
