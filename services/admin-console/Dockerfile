# SPDX-License-Identifier: Apache-2.0
# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Copy shared packages
COPY pkg/ ./pkg/

# Copy service module files and download deps
COPY services/admin-console/go.mod services/admin-console/go.sum* ./services/admin-console/
WORKDIR /build/services/admin-console
RUN go mod download

# Copy service source code
WORKDIR /build
COPY services/admin-console/cmd ./services/admin-console/cmd
COPY services/admin-console/internal ./services/admin-console/internal
COPY services/admin-console/web ./services/admin-console/web

# Build the application
WORKDIR /build/services/admin-console
RUN CGO_ENABLED=0 GOOS=linux GOFLAGS='-p=1' go build -o admin-console ./cmd/admin-console

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY --from=builder /build/services/admin-console/admin-console ./admin-console
COPY --from=builder /build/services/admin-console/web ./web

EXPOSE 9999
CMD ["./admin-console"]
