# SPDX-License-Identifier: Apache-2.0
# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Copy shared packages
COPY pkg/ ./pkg/

# Copy service module files and download deps
COPY services/cloudfauxnt/go.mod services/cloudfauxnt/go.sum* ./services/cloudfauxnt/
WORKDIR /build/services/cloudfauxnt
RUN go mod download

# Copy service source code
WORKDIR /build
COPY services/cloudfauxnt/ ./services/cloudfauxnt/

# Build the application
WORKDIR /build/services/cloudfauxnt
RUN CGO_ENABLED=0 GOOS=linux go build -o cloudfauxnt ./cmd/cloudfauxnt

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/services/cloudfauxnt/cloudfauxnt .

# Copy license files
COPY services/cloudfauxnt/LICENSE services/cloudfauxnt/NOTICE ./

# Create directories for config and keys
RUN mkdir -p /app/config /app/keys

# Expose port
EXPOSE 9310

# Run the application
CMD ["./cloudfauxnt", "--config", "/app/config/cloudfauxnt.config.yaml"]
