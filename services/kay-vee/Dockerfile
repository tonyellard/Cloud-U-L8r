# SPDX-License-Identifier: Apache-2.0
# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Copy shared packages
COPY pkg/ ./pkg/

# Copy service module files and download deps
COPY services/kay-vee/go.mod ./services/kay-vee/
WORKDIR /build/services/kay-vee
RUN go mod download

# Copy service source code
WORKDIR /build
COPY services/kay-vee/ ./services/kay-vee/

# Build binary
WORKDIR /build/services/kay-vee
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o kay-vee ./cmd/kay-vee

# Runtime stage
FROM alpine:latest

WORKDIR /app

RUN apk --no-cache add ca-certificates

COPY --from=builder /build/services/kay-vee/kay-vee ./kay-vee

EXPOSE 9350

ENTRYPOINT ["./kay-vee"]
