# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Copy module metadata
COPY go.mod ./
RUN go mod download

# Copy source
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o kay-vee ./cmd/kay-vee

# Runtime stage
FROM alpine:latest

WORKDIR /app

RUN apk --no-cache add ca-certificates

COPY --from=builder /build/kay-vee ./kay-vee

EXPOSE 9350

ENTRYPOINT ["./kay-vee"]
