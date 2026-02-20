# SPDX-License-Identifier: Apache-2.0
# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Install dependencies
RUN apk add --no-cache git

# Copy shared packages
COPY pkg/ ./pkg/

# Copy service module files and download deps
COPY services/ess-enn-ess/go.mod services/ess-enn-ess/go.sum ./services/ess-enn-ess/
WORKDIR /build/services/ess-enn-ess
RUN go mod download

# Copy service source code
WORKDIR /build
COPY services/ess-enn-ess/ ./services/ess-enn-ess/

# Build the application
WORKDIR /build/services/ess-enn-ess
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o ess-enn-ess ./cmd/ess-enn-ess

# Runtime stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy binary from builder
COPY --from=builder /build/services/ess-enn-ess/ess-enn-ess .

# Create data directory
RUN mkdir -p /app/data

# Expose port
EXPOSE 9330

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost:9330/health || exit 1

# Run the application
ENTRYPOINT ["./ess-enn-ess"]
CMD ["-config", "/app/config/ess-enn-ess.config.yaml"]
