# ess-enn-ess: AWS SNS Emulator

A lightweight, developer-friendly AWS SNS (Simple Notification Service) emulator built in Go for local development and testing. Part of the cloud-u-l8r emulator suite.

## Overview

**ess-enn-ess** provides a local SNS implementation for developing and testing AWS-based applications without using actual AWS services. It implements core SNS functionality including topics, subscriptions, message publishing, and delivery to multiple protocols (HTTP, SQS, Lambda simulation).

### Key Features

- **Topic Management**: Create, delete, list, and manage SNS topics
- **Flexible Subscriptions**: Support for multiple subscription protocols (HTTP, SQS, Email simulation)
- **Message Publishing**: Publish messages to topics with automatic distribution to subscribers
- **Activity Logging**: Real-time activity logging with filtering and querying capabilities
- **Admin Dashboard**: Web-based admin UI for monitoring and managing topics/subscriptions
- **SQS Integration**: Native integration with ess-queue-ess for SQS subscriptions
- **Developer-First**: No authentication, verbose errors, and auto-confirming subscriptions in dev mode
- **Configuration-Driven**: YAML-based configuration for reproducible setups

## Quick Start

### Prerequisites

- Docker and Docker Compose
- Or: Go 1.21+

### Using Docker Compose

```bash
cd /home/tony/Documents/cloud-u-l8r

# Start all services including SNS emulator
docker-compose up -d

# Verify SNS is running
curl http://localhost:9330/health
```

### Manual Compilation

```bash
cd services/ess-enn-ess

# Build
go build -o ess-enn-ess ./cmd/ess-enn-ess

# Run
./ess-enn-ess -config config/config.yaml
```

## Configuration

Configuration is done via YAML file. An example configuration is provided at `config/config.example.yaml`.

### Key Configuration Options

```yaml
server:
  api_port: 9330        # SNS API port
  admin_port: 9331      # Admin UI port
  host: "0.0.0.0"

sqs:
  enabled: true
  endpoint: "http://ess-queue-ess:9320"  # Connect to ess-queue-ess

activity_log:
  enabled: true
  stream_to_admin_ui: true  # Real-time updates to dashboard

developer:
  no_auth: true               # No authentication
  auto_confirm_subscriptions: true  # Auto-confirm for dev
```

## API Endpoints

### CreateTopic

Creates a new SNS topic.

```bash
curl -X POST http://localhost:9330/ \
  -d "Action=CreateTopic" \
  -d "Name=my-topic"
```

**Parameters:**
- `Name` (required): Topic name
- `FifoTopic` (optional): Set to "true" for FIFO topics
- `ContentBasedDeduplication` (optional): For FIFO topics

**Response:** Returns `TopicArn` (e.g., `arn:aws:sns:us-east-1:123456789012:my-topic`)

### DeleteTopic

Deletes an SNS topic.

```bash
curl -X POST http://localhost:9330/ \
  -d "Action=DeleteTopic" \
  -d "TopicArn=arn:aws:sns:us-east-1:123456789012:my-topic"
```

### ListTopics

Lists all topics.

```bash
curl -X POST http://localhost:9330/ \
  -d "Action=ListTopics"
```

### GetTopicAttributes

Gets attributes of a topic.

```bash
curl -X POST http://localhost:9330/ \
  -d "Action=GetTopicAttributes" \
  -d "TopicArn=arn:aws:sns:us-east-1:123456789012:my-topic"
```

### SetTopicAttributes

Sets an attribute on a topic.

```bash
curl -X POST http://localhost:9330/ \
  -d "Action=SetTopicAttributes" \
  -d "TopicArn=arn:aws:sns:us-east-1:123456789012:my-topic" \
  -d "AttributeName=DisplayName" \
  -d "AttributeValue=My Topic Display Name"
```

## Admin Dashboard

Access the admin dashboard at `http://localhost:9331` to:

- View all topics and subscriptions
- Monitor message publishing and delivery
- View real-time activity logs
- Inspect message details and delivery failures
- Manage topic attributes
- Test message publishing

## Integration with Other Services

### SQS Integration (ess-queue-ess)

When a subscription is created to an SQS queue:

```bash
curl -X POST http://localhost:9330/ \
  -d "Action=Subscribe" \
  -d "TopicArn=arn:aws:sns:us-east-1:123456789012:my-topic" \
  -d "Protocol=sqs" \
  -d "Endpoint=arn:aws:sqs:us-east-1:123456789012:my-queue"
```

Messages published to the topic will be automatically delivered to the specified SQS queue via the ess-queue-ess service.

### S3 Integration (essthree)

Currently, S3 event notifications are being implemented. Once complete, S3 bucket events can trigger SNS notifications.

## Architecture

### Core Components

- **Topic Store** (`internal/topic/`): Thread-safe in-memory storage for topics
- **Activity Logger** (`internal/activity/`): Real-time activity logging with subscriber pattern
- **Subscription Manager** (planned): Manages topic subscriptions
- **Publisher** (planned): Handles message publishing and distribution
- **Delivery Engine** (planned): Routes messages to various protocols
- **Admin Server** (planned): Web dashboard for monitoring

### Activity Logging

The activity logger records:

- Topic CRUD operations
- Message publishing
- Subscription operations
- Message delivery attempts
- Errors and retries
- HTTP/SQS integration events

Real-time subscribers (like the admin UI) are notified immediately of new activities.

## Development

### Adding New Handlers

To add a new SNS API handler, edit `internal/server/server.go`:

```go
case "YourAction":
    s.handleYourAction(w, r, start)
```

Then implement the handler method following the pattern of existing handlers.

### Running Tests

```bash
cd services/ess-enn-ess
go test ./...
```

### Building Docker Image

```bash
cd services/ess-enn-ess
docker build -t cloud-u-l8r/ess-enn-ess:latest .
```

## Troubleshooting

### Connection Refused

Ensure the service is running and the port is correct:

```bash
curl http://localhost:9330/health
```

### SQS Delivery Failures

Check that ess-queue-ess is running and accessible:

```bash
curl http://ess-queue-ess:9320/health
```

Check activity logs in the admin dashboard for delivery errors.

### Configuration Issues

Validate the YAML configuration:

```bash
go run ./cmd/ess-enn-ess -config ./config/config.yaml
```

## Implementation Status

### Completed (Phase 1)

- ✅ Core topic management (CreateTopic, DeleteTopic, ListTopics, etc.)
- ✅ Activity logging system
- ✅ Configuration system
- ✅ HTTP server setup
- ✅ API handlers for topic operations
- ✅ Docker setup

### In Progress (Phase 2+)

- 🔄 Subscription management (Phase 3)
- 🔄 Message publishing (Phase 4)
- 🔄 SQS delivery integration (Phase 5)
- 🔄 Admin dashboard UI (Phase 2)
- 🔄 HTTP subscription delivery (Phase 6)
- 🔄 Advanced features (FIFO, DLQ, message attributes)

## Monitoring

### Health Check

```bash
curl http://localhost:9330/health
```

### Activity Log Access

The activity logger stores up to 10,000 entries in memory by default, configurable via `storage.activity_log_size` in the config.

Access logs via:
- Admin dashboard: `http://localhost:9331/logs`
- File system: `./data/activity.log` (if file logging is enabled)

## Performance Considerations

- **In-Memory Storage**: All data is stored in memory; not persisted across restarts
- **Concurrent Access**: Thread-safe with RWMutex for all data structures
- **Activity Log**: Limited to configured `activity_log_size` (default 10,000 entries)
- **Message Size**: Limited to 262KB (AWS SNS default)

## Contributing

Contributions welcome! Please ensure:

- Code follows existing patterns
- Tests are included for new features
- Activity logging is integrated for all new operations
- Documentation is updated

## Related Services

- **essthree**: S3 emulator (port 9300)
- **cloudfauxnt**: CloudFront emulator (port 9310)
- **ess-queue-ess**: SQS emulator (port 9320)

## License

Part of the cloud-u-l8r project. See root repository for license details.
