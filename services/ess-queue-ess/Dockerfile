# SPDX-License-Identifier: Apache-2.0
# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Copy shared packages
COPY pkg/ ./pkg/

# Copy service module files and download deps
COPY services/ess-queue-ess/go.mod services/ess-queue-ess/go.sum* ./services/ess-queue-ess/
WORKDIR /build/services/ess-queue-ess
RUN go mod download

# Copy service source code
WORKDIR /build
COPY services/ess-queue-ess/ ./services/ess-queue-ess/

# Build the application
WORKDIR /build/services/ess-queue-ess
RUN CGO_ENABLED=0 GOOS=linux go build -o ess-queue-ess ./cmd/ess-queue-ess

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/services/ess-queue-ess/ess-queue-ess .

# Copy license files
COPY services/ess-queue-ess/LICENSE services/ess-queue-ess/NOTICE ./

# Create directory for queue data and config
RUN mkdir -p /app/data /app/config

# Expose default SQS port
EXPOSE 9320

# Run the application
CMD ["./ess-queue-ess", "--config", "/app/config/ess-queue-ess.config.yaml"]
