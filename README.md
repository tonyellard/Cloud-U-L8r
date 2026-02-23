# Cloud-U-L8r

A unified development stack for local AWS service emulation, providing S3, SQS, SNS, CloudFront-like, and Parameter/Secrets capabilities in a single orchestrated environment.

## Services

This monorepo contains six interconnected services:

- **ess-three** (Port 9300) - S3-compatible object storage emulator
- **cloudfauxnt** (Port 9310) - CloudFront-like CDN emulator with signed URL support
- **ess-queue-ess** (Port 9320) - SQS-compatible message queue emulator with FIFO and DLQ support
- **ess-enn-ess** (Port 9330) - SNS-compatible notification service emulator
- **kay-vee** (Port 9350) - Combined SSM Parameter Store + Secrets Manager emulator
- **admin-console** (Port 9999) - Consolidated operator console for dashboard + per-service administration

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

- **S3 (ess-three)**: `http://localhost:9300`
- **CloudFront (cloudfauxnt)**: `http://localhost:9310`
- **SQS (ess-queue-ess)**: `http://localhost:9320`
- **SNS (ess-enn-ess)**: `http://localhost:9330` (Admin UI: `http://localhost:9331`)
- **kay-vee**: `http://localhost:9350`
- **Admin Console**: `http://localhost:9999`

For inter-container communication, services use the internal `shared-network`:
- `http://essthree:9300`
- `http://cloudfauxnt:9310`
- `http://ess-queue-ess:9320`
- `http://ess-enn-ess:9330` (Admin UI: `http://ess-enn-ess:9331`)
- `http://kay-vee:9350`
- `http://admin-console:9999`

## Port Scheme

All services use the 93xx port range with 10-port increments:
- **9300**: S3 Storage
- **9310**: CloudFront CDN
- **9320**: SQS Queue
- **9330**: SNS Notifications (9331 for Admin UI)
- **9350**: Parameter + Secrets Emulator (`kay-vee`)
- **9999**: Consolidated Admin Console

## Configuration

Services are configured via **environment variables** (set in `docker-compose.yml`).

AWS resources (buckets, queues, topics, subscriptions) are provisioned via **Terraform**
configs in `configs/[config-name]/`. The default config creates baseline dev resources:

```bash
# Start services and apply the default Terraform config
make stack

# Or apply a config manually against running services
make run-config CONFIG=default
```

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

This repository uses Go workspaces to manage all service modules:

```bash
# Workspace is already initialized, just use Go commands normally
go work use ./services/admin-console ./services/essthree ./services/cloudfauxnt ./services/ess-queue-ess ./services/ess-enn-ess
```

## Documentation

- [ess-three Documentation](services/essthree/README.md)
- [cloudfauxnt Documentation](services/cloudfauxnt/README.md)
- [ess-queue-ess Documentation](services/ess-queue-ess/README.md)
- [ess-enn-ess Documentation](services/ess-enn-ess/README.md)
- [kay-vee Documentation](services/kay-vee/README.md)
- [admin-console Documentation](services/admin-console/README.md)

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome! Please see [CONTRIBUTORS.md](CONTRIBUTORS.md) for guidelines.

## Authors

See [AUTHORS](AUTHORS) for a list of contributors.

## Trademark Notice

This project is not affiliated with, endorsed by, or sponsored by Amazon Web Services (AWS). Amazon S3, Amazon CloudFront, Amazon Secrets Manager, Amazon Parameter Store are all trademarks of amazon.com, Inc., or it's affiliates.
