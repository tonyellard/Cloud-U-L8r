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

# Run (uses centralized config)
./ess-enn-ess -config ../../config/ess-enn-ess.config.yaml
```

## Configuration

The SNS emulator uses a centralized YAML configuration file located at `/config/ess-enn-ess.config.yaml` (mounted read-only in Docker containers).

### Configuration Structure

The streamlined configuration includes only essential user-configurable settings:

```yaml
sqs:
  enabled: true
  endpoint: "http://ess-queue-ess:9320"

http:
  enabled: true
  max_retries: 3
  retry_backoff_ms: 100

admin:
  enabled: true

storage:
  activity_log_size: 10000

aws:
  account_id: "123456789012"
  region: "us-east-1"
```

### Configuration Fields

- **SQS**: Enable/disable SQS integration and configure endpoint
- **HTTP**: Enable/disable HTTP subscriptions and retry behavior
- **Admin**: Enable/disable admin dashboard
- **Storage**: Activity log size (circular buffer)
- **AWS**: Account ID and region for ARN generation

### State Persistence & Export/Import

When you export the configuration through the admin dashboard (`/api/export`), it creates a complete backup including:
- **Configuration**: All config settings
- **Topics**: All created topics
- **Subscriptions**: All subscriptions with their current state

When the service restarts, it automatically loads any topics and subscriptions from the config file, enabling reproducible development environments.

**To export:**
1. Go to Admin Dashboard → Export/Import tab
2. Click "Download Export"
3. Save the YAML file as your backup

**To restore:**
Simply replace the config file with the exported YAML before restarting the service.

## Admin Dashboard

The SNS emulator includes a fully-functional web-based admin dashboard for monitoring and managing topics and subscriptions in real-time. The dashboard features real-time statistics, subscription management, activity logging, and comprehensive data visualization.

### Accessing the Dashboard

After starting the emulator, open your browser to:

```
http://localhost:9331
```

### Dashboard Features

#### Statistics Cards
- **Topics**: Count of created topics
- **Subscriptions**: Total subscriptions with confirmed count
- **Messages Published**: Total published message count
- **Deliveries**: Successful + failed delivery tracking
- **Total Events**: Activity log entry count
- Auto-refreshes every 3 seconds

#### Topics Tab
View all SNS topics with:
- Topic ARN
- Display name
- Topic type badge (FIFO or Standard)
- Subscription count
- Creation timestamp
- Click refresh to reload

#### Subscriptions Tab
View all subscriptions with:
- Subscription ARN
- Topic ARN
- Protocol badge (HTTP, SQS, Email, Lambda)
- Endpoint
- Status badge (Confirmed, Pending)
- Creation timestamp
- Filter by topic using query parameter
- Click refresh to reload

#### Activity Log Tab
Real-time monitoring of all SNS operations:
- Event type (CreateTopic, Subscribe, Publish, Deliver, etc.)
- Operation status (Success/Failed/Retrying)
- Topic and message identifiers
- Execution duration in milliseconds
- Error messages for failed operations
- Auto-scrolls to latest events
- Reverse chronological order (newest first)
- Auto-refreshes every 3 seconds

#### Export/Import Tab
- Download topics and subscriptions as YAML
- Import feature coming soon

### Admin API Endpoints

#### GET /api/topics

Returns all topics with metadata.

```bash
curl http://localhost:9331/api/topics
```

**Response:**
```json
[
  {
    "topic_arn": "arn:aws:sns:us-east-1:123456789012:my-topic",
    "display_name": "my-topic",
    "fifo_topic": false,
    "content_based_deduplication": false,
    "created_at": "2026-02-10T18:22:41Z",
    "subscription_count": 2
  }
]
```

#### GET /api/subscriptions

Returns all subscriptions with optional filtering by topic.

**Query Parameters:**
- `topic` (optional): Filter by topic ARN

```bash
# Get all subscriptions
curl http://localhost:9331/api/subscriptions

# Filter by topic
curl 'http://localhost:9331/api/subscriptions?topic=arn:aws:sns:us-east-1:123456789012:my-topic'
```

**Response:**
```json
[
  {
    "subscription_arn": "arn:aws:sns:us-east-1:123456789012:my-topic:11111111-1111-1111-1111-111111111111",
    "topic_arn": "arn:aws:sns:us-east-1:123456789012:my-topic",
    "protocol": "http",
    "endpoint": "http://example.com/webhook",
    "status": "confirmed",
    "created_at": "2026-02-10T18:22:41Z"
  }
]
```

#### GET /api/activities

Returns recent activity log entries with optional filtering.

**Query Parameters:**
- `topic` (optional): Filter by topic ARN
- `event_type` (optional): Filter by event type (create_topic, subscribe, publish, etc.)
- `status` (optional): Filter by status (success, failed, pending, retrying)
- `limit` (optional): Maximum entries to return (default: 100)

```bash
# Get all activities
curl http://localhost:9331/api/activities

# Filter by topic
curl 'http://localhost:9331/api/activities?topic=arn:aws:sns:us-east-1:123456789012:my-topic'

# Filter by event type and status
curl 'http://localhost:9331/api/activities?event_type=publish&status=success'
```

**Response:**
```json
[
  {
    "id": "1770747761962385239",
    "timestamp": "2026-02-10T18:22:41Z",
    "event_type": "publish",
    "topic_arn": "arn:aws:sns:us-east-1:123456789012:my-topic",
    "status": "success",
    "message_id": "11111111-1111-1111-1111-111111111111",
    "duration_ms": 125,
    "error": ""
  }
]
```

#### GET /api/stats

Returns aggregated statistics for dashboard.

```bash
curl http://localhost:9331/api/stats
```

**Response:**
```json
{
  "topics": {
    "total": 2
  },
  "subscriptions": {
    "total": 3,
    "confirmed": 3,
    "pending": 0
  },
  "messages": {
    "published": 2,
    "delivered": 4,
    "failed": 1
  },
  "events": {
    "total": 12
  }
}
```

#### GET /api/export

Exports all topics, subscriptions, and configuration as YAML format. This export can be used to backup and restore the complete SNS state.

```bash
curl http://localhost:9331/api/export > sns-backup.yaml
```

**Response:** Complete YAML backup including:
- Configuration settings
- All topics
- All subscriptions with full state

## Admin Dashboard Export & Restore

The admin dashboard includes export functionality for complete state backups:

1. **Export**: Go to Admin Dashboard → Export/Import tab → "Download Export"
   - Creates a complete YAML backup of config + topics + subscriptions
   - Can be used to restore state after container restart

2. **Automatic Restore on Startup**: The service automatically loads any topics and subscriptions from the config file when it starts

This enables reproducible development environments where your topics and subscriptions persist across restarts.

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
go run ./cmd/ess-enn-ess -config ../../config/ess-enn-ess.config.yaml
```

## Implementation Status

### Completed (Phase 1)

- ✅ Core topic management (CreateTopic, DeleteTopic, ListTopics, etc.)
- ✅ Activity logging system
- ✅ Configuration system
- ✅ HTTP server setup
- ✅ API handlers for topic operations
- ✅ Docker setup

### Completed (Phase 3-5)

- ✅ Subscription management (Phase 3)
  - Subscribe/Unsubscribe actions
  - List subscriptions by topic
  - Get/Set subscription attributes
  - Multiple protocol support (HTTP, SQS, Email, Lambda)
- ✅ Message publishing (Phase 4)
  - Publish action with message distribution
  - Asynchronous delivery to confirmed subscribers
  - Protocol-specific delivery handlers
  - Activity logging for publish and delivery events
- ✅ SQS delivery integration (Phase 5)
  - Real SQS message delivery via ess-queue-ess
  - SNS notification format wrapping
  - Configurable SQS endpoint
  - Error handling and retry support
- ✅ HTTP delivery enhancements (Phase 6)
  - Automatic retry with exponential backoff
  - Configurable max retries and backoff timing
  - Smart error categorization (transient vs permanent)
  - Enhanced delivery logging with retry tracking
  - Status code-based retry decisions (5xx, 429, 408)

### Completed (Phase 7 - Admin Dashboard)

- ✅ Comprehensive web-based admin dashboard
  - Multi-tab interface (Topics, Subscriptions, Activity Log, Export)
  - Real-time statistics with auto-refresh every 3 seconds
  - Subscription management with protocol and status display
  - Complete activity logging with event filtering
  - YAML export functionality
  - Responsive design with modern UI
  - Auto-fetch of all API data (topics, subscriptions, activities, stats)

### In Progress (Phase 8+)

- 🔄 Advanced features (FIFO, DLQ, message attributes)
- 🔄 Subscription filtering in admin UI
- 🔄 Import functionality for YAML configuration

## Testing

### Integration Tests

**Test message publishing:**
```bash
cd services/ess-enn-ess
./test_publish.sh
```

**Test SQS integration:**
```bash
# Requires both ess-enn-ess and ess-queue-ess running
./test_sqs_integration.sh

# Or use the quick test (starts both services):
./test_sqs_quick.sh
```

**Test HTTP retry logic:**
```bash
# Requires ess-enn-ess running
./test_http_retry.sh
```

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

Licensed under the Apache License, Version 2.0. See the root [LICENSE](../../LICENSE) file for details.
