# SPDX-License-Identifier: Apache-2.0
# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Copy shared packages
COPY pkg/ ./pkg/

# Copy service module files and download deps
COPY services/essthree/go.mod services/essthree/go.sum ./services/essthree/
WORKDIR /build/services/essthree
RUN go mod download

# Copy service source code
WORKDIR /build
COPY services/essthree/ ./services/essthree/

# Build the application
WORKDIR /build/services/essthree
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ess-three ./cmd/ess-three

# Final stage
FROM alpine:3.21

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /build/services/essthree/ess-three .

# Copy license files
COPY services/essthree/LICENSE services/essthree/NOTICE ./

# Create data directory
RUN mkdir -p /data

# Expose port
EXPOSE 9300

# Run the application
CMD ["./ess-three", "--port=9300", "--data-dir=/data"]
