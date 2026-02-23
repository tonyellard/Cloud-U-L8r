# pipes - Local EventBridge Pipes Emulator

`pipes` is a local emulator for AWS EventBridge Pipes, providing point-to-point integrations between SQS sources and SQS, SNS, HTTP, or EventBridge targets with optional filtering and enrichment. Part of the cloud-u-l8r emulator suite.

## Features

- REST API matching the AWS EventBridge Pipes resource model (`/v1/pipes/{name}`)
- SQS source polling with configurable batch size
- Multiple target types: SQS, SNS, HTTP/HTTPS, and EventBridge
- EventBridge event pattern filtering on source messages
- HTTP-based enrichment with input templates
- Pipe lifecycle management (start/stop/update/delete)
- Resource tagging (create-time and post-creation)
- In-memory storage for fast local development
- Activity logging for all API calls

## Quick Start

### Local

```bash
cd services/pipes
go run ./cmd/pipes
```

Service default endpoint: `http://localhost:9344`

Health check:

```bash
curl http://localhost:9344/health
```

### Docker

```bash
docker build -t pipes .
docker run --rm -p 9344:9344 pipes
```

## Configuration

Pipes is configured via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `9344` | Server listen port |
| `REGION` | `us-east-1` | AWS region for ARN generation |
| `ACCOUNT_ID` | `000000000000` | AWS account ID for ARN generation |
| `SQS_ENDPOINT` | `http://ess-queue-ess:9320` | SQS service endpoint for source polling and SQS targets |
| `SNS_ENDPOINT` | `http://ess-enn-ess:9330` | SNS service endpoint for SNS targets |
| `EVENTBRIDGE_ENDPOINT` | `http://drawbridge:9340` | EventBridge endpoint for event bus targets |

## Supported Operations

### Pipe CRUD

- `POST /v1/pipes/{name}` — create a pipe
- `GET /v1/pipes/{name}` — describe a pipe
- `PUT /v1/pipes/{name}` — update a pipe
- `DELETE /v1/pipes/{name}` — delete a pipe
- `GET /v1/pipes` — list pipes (supports `NamePrefix`, `SourcePrefix`, `TargetPrefix`, `CurrentState`, `DesiredState`, `Limit`)

### Pipe Control

- `POST /v1/pipes/{name}/start` — start a stopped pipe
- `POST /v1/pipes/{name}/stop` — stop a running pipe

### Tagging

- `POST /tags/{arn}` — add or update tags on a resource
- `GET /tags/{arn}` — list tags for a resource
- `DELETE /tags/{arn}?tagKeys=key1&tagKeys=key2` — remove tags from a resource

## Example Requests

### Create a pipe (SQS to SQS)

```bash
curl -s -X POST http://localhost:9344/v1/pipes/order-pipe \
  -H 'Content-Type: application/json' \
  -d '{
    "Source": "arn:aws:sqs:us-east-1:000000000000:incoming-orders",
    "Target": "arn:aws:sqs:us-east-1:000000000000:processed-orders",
    "RoleArn": "arn:aws:iam::000000000000:role/pipe-role",
    "DesiredState": "RUNNING",
    "SourceParameters": {
      "SqsQueueParameters": {
        "BatchSize": 5
      }
    }
  }'
```

### Create a pipe with filtering

```bash
curl -s -X POST http://localhost:9344/v1/pipes/priority-pipe \
  -H 'Content-Type: application/json' \
  -d '{
    "Source": "arn:aws:sqs:us-east-1:000000000000:all-orders",
    "Target": "arn:aws:sqs:us-east-1:000000000000:priority-orders",
    "RoleArn": "arn:aws:iam::000000000000:role/pipe-role",
    "DesiredState": "RUNNING",
    "SourceParameters": {
      "FilterCriteria": {
        "Filters": [
          {"Pattern": "{\"body\": {\"priority\": [\"high\"]}}"}
        ]
      }
    }
  }'
```

### Create a pipe with enrichment

```bash
curl -s -X POST http://localhost:9344/v1/pipes/enriched-pipe \
  -H 'Content-Type: application/json' \
  -d '{
    "Source": "arn:aws:sqs:us-east-1:000000000000:raw-events",
    "Target": "arn:aws:sqs:us-east-1:000000000000:enriched-events",
    "RoleArn": "arn:aws:iam::000000000000:role/pipe-role",
    "DesiredState": "RUNNING",
    "Enrichment": "http://my-service:8080/enrich",
    "EnrichmentParameters": {
      "HttpParameters": {
        "HeaderParameters": {"X-Api-Key": "secret"}
      }
    }
  }'
```

### Create a pipe targeting EventBridge

```bash
curl -s -X POST http://localhost:9344/v1/pipes/to-eventbridge \
  -H 'Content-Type: application/json' \
  -d '{
    "Source": "arn:aws:sqs:us-east-1:000000000000:events-queue",
    "Target": "arn:aws:events:us-east-1:000000000000:event-bus/orders",
    "RoleArn": "arn:aws:iam::000000000000:role/pipe-role",
    "DesiredState": "RUNNING",
    "TargetParameters": {
      "EventBridgeEventBusParameters": {
        "Source": "myapp.orders",
        "DetailType": "OrderReceived"
      }
    }
  }'
```

### List and manage pipes

```bash
# List all pipes
curl -s http://localhost:9344/v1/pipes

# Filter by state
curl -s 'http://localhost:9344/v1/pipes?CurrentState=RUNNING'

# Stop a pipe
curl -s -X POST http://localhost:9344/v1/pipes/order-pipe/stop

# Start a pipe
curl -s -X POST http://localhost:9344/v1/pipes/order-pipe/start

# Delete a pipe
curl -s -X DELETE http://localhost:9344/v1/pipes/order-pipe
```

## Event Filtering

Pipes support EventBridge event pattern filtering on source messages. For SQS sources, the message body is wrapped as `{"body": <message>}` before pattern evaluation.

Supported pattern operators:

- **Exact match** — `{"body": ["value"]}`
- **Prefix** — `{"body": [{"prefix": "order-"}]}`
- **Suffix** — `{"body": [{"suffix": ".json"}]}`
- **Anything-but** — `{"body": [{"anything-but": "internal"}]}`
- **Numeric** — `{"body": {"count": [{"numeric": [">=", 10]}]}}`
- **Exists** — `{"body": {"key": [{"exists": true}]}}`

Messages that don't match any filter are deleted from the source queue (filtered out). Multiple filters in the `Filters` array are OR'd together.

## Enrichment

When a pipe has an `Enrichment` URL configured, each message is POSTed to that endpoint before delivery to the target. The enrichment response replaces the original message body.

- The `EnrichmentParameters.InputTemplate` field can transform the payload sent to the enrichment endpoint
- Custom headers and query parameters can be passed via `EnrichmentParameters.HttpParameters`
- Enrichment requests have a 10-second timeout

## Target Delivery

The background poller runs every 5 seconds and processes messages from all running pipes:

| Target Type | Detection | Protocol |
|-------------|-----------|----------|
| **SQS** | ARN contains `:sqs:` | Form-encoded `SendMessage` (supports `MessageGroupId` and `MessageDeduplicationId` for FIFO) |
| **SNS** | ARN contains `:sns:` | Form-encoded `Publish` |
| **HTTP/HTTPS** | ARN starts with `http://` or `https://` | JSON POST with custom headers/query params |
| **EventBridge** | ARN contains `:events:` or `:event-bus/` | JSON `PutEvents` via drawbridge (defaults: source `pipes`, detail-type `PipeForwarded`) |

Target parameters (`TargetParameters.InputTemplate`) can transform the payload before delivery.

## Processing Flow

```
SQS Source → Filter → Enrich → Transform → Deliver → Delete from source
                ↓
        (no match: delete)
```

1. **Poll** — receive messages from the SQS source queue
2. **Filter** — evaluate `FilterCriteria` patterns; non-matching messages are deleted
3. **Enrich** — POST to enrichment URL if configured; response replaces the message
4. **Transform** — apply `TargetParameters.InputTemplate` if configured
5. **Deliver** — send to the target (SQS, SNS, HTTP, or EventBridge)
6. **Delete** — remove the message from the source queue on success

## Admin Endpoints

- `GET /admin/api/summary` — pipe and running pipe counts
- `GET /admin/api/resources` — detailed pipe list with state, source, target, and filter info
- `GET /admin/api/activity` — paginated activity log (supports `maxResults` and `nextToken`)

## Architecture

### Project Structure

```
pipes/
├── cmd/pipes/main.go                # Entry point and config loading
├── internal/
│   ├── server/server.go             # HTTP routing and API handlers
│   ├── model/types.go               # Pipe, target, filter, and enrichment types
│   ├── store/store.go               # Thread-safe in-memory storage
│   ├── poller/poller.go             # Background SQS poller (5-sec tick)
│   └── delivery/deliver.go          # Target delivery (SQS/SNS/HTTP/EventBridge)
├── Dockerfile
└── go.mod
```

### Storage

All data is stored in-memory using a thread-safe store (`sync.RWMutex`). Tags are stored separately keyed by ARN. Data does not persist across restarts.

### Error Responses

Errors follow the AWS JSON protocol format:

```json
{
  "__type": "ConflictException",
  "message": "Pipe 'order-pipe' already exists."
}
```

Common error types: `ValidationException`, `NotFoundException`, `ConflictException`.

## Testing

```bash
cd services/pipes
go test ./...
```

Unit tests cover pipe CRUD, state transitions, list filtering, tag operations, admin endpoints, parameter handling, and error conditions.

## Notes

- This emulator intentionally prioritizes local dev compatibility over strict AWS parity.
- Only SQS sources are currently supported; the poller polls every 5 seconds.
- Pipe names must be 1-64 characters matching `[.\-_A-Za-z0-9]+`.
- `RoleArn` is required on create but not enforced (no IAM emulation).
- Messages that fail enrichment or delivery remain in the source queue for retry on the next poll cycle.

## License

Licensed under the Apache License, Version 2.0. See the root [LICENSE](../../LICENSE) file for details.

## Trademark Notice

This project is not affiliated with, endorsed by, or sponsored by Amazon Web Services (AWS). Amazon EventBridge Pipes is a trademark of Amazon.com, Inc., or its affiliates.
