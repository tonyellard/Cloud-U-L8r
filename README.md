# Cloud-U-L8r

A unified development stack for local AWS service emulation, providing S3, SQS, SNS, and CloudFront-like capabilities in a single orchestrated environment.

## Services

This monorepo contains four interconnected services:

- **essthree** (Port 9300) - S3-compatible object storage emulator
- **cloudfauxnt** (Port 9310) - CloudFront-like CDN emulator with signed URL support
- **ess-queue-ess** (Port 9320) - SQS-compatible message queue emulator with FIFO and DLQ support
- **ess-enn-ess** (Port 9330) - SNS-compatible notification service emulator

## Quick Start

### Prerequisites
- Docker and Docker Compose
- Go 1.23+ (for local development)

### Running All Services

```bash
# Start all services
make up

# View logs
make logs

# Stop all services
make down
```

### Service Endpoints

Once running, services are available at:

- **S3 (essthree)**: `http://localhost:9300`
- **CloudFront (cloudfauxnt)**: `http://localhost:9310`
- **SQS (ess-queue-ess)**: `http://localhost:9320`
- **SNS (ess-enn-ess)**: `http://localhost:9330` (Admin UI: `http://localhost:9331`)

For inter-container communication, services use the internal `shared-network`:
- `http://essthree:9300`
- `http://cloudfauxnt:9310`
- `http://ess-queue-ess:9320`
- `http://ess-enn-ess:9330` (Admin UI: `http://ess-enn-ess:9331`)

## Port Scheme

All services use the 93xx port range with 10-port increments:
- **9300**: S3 Storage
- **9310**: CloudFront CDN
- **9320**: SQS Queue
- **9330**: SNS Notifications (9331 for Admin UI)

## Configuration

All service configs live in the root `config/` directory using the naming convention
`[service].config.yaml`:
- config/ess-enn-ess.config.yaml - SNS configuration
- config/ess-queue-ess.config.yaml - SQS configuration
- config/cloudfauxnt.config.yaml - CDN configuration

## Development

### Building Services

```bash
# Build all services
make build

# Build individual service
docker compose build essthree
```

### Running Tests

```bash
# Run all tests
make test
```

### Go Workspace

This repository uses Go workspaces to manage all three services:

```bash
# Workspace is already initialized, just use Go commands normally
go work use ./services/essthree ./services/cloudfauxnt ./services/ess-queue-ess
```

## Documentation

- [essthree Documentation](services/essthree/README.md)
- [cloudfauxnt Documentation](services/cloudfauxnt/README.md)
- [ess-queue-ess Documentation](services/ess-queue-ess/README.md)

## License

See individual service directories for license information.
