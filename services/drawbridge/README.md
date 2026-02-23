# drawbridge - Local EventBridge Emulator

`drawbridge` is a local emulator for AWS EventBridge, supporting event buses, rules with event pattern matching, scheduled rules, and target delivery to SQS, SNS, and HTTP endpoints. Part of the cloud-u-l8r emulator suite.

## Features

- AWS-style JSON RPC over `X-Amz-Target` (`AWSEvents.*` actions)
- Event buses with a pre-created `default` bus
- Rules with EventBridge event pattern matching or schedule expressions
- Up to 5 targets per rule with SQS, SNS, and HTTP delivery
- Input transformations (static override, JSONPath, templates)
- Background scheduler for `rate()` and `cron()` based rules
- `TestEventPattern` endpoint for pattern validation without side-effects
- In-memory storage for fast local development
- Activity logging for all API calls

## Quick Start

### Local

```bash
cd services/drawbridge
go run ./cmd/drawbridge
```

Service default endpoint: `http://localhost:9340`

Health check:

```bash
curl http://localhost:9340/health
```

### Docker

```bash
docker build -t drawbridge .
docker run --rm -p 9340:9340 drawbridge
```

## Configuration

Drawbridge is configured via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `9340` | Server listen port |
| `REGION` | `us-east-1` | AWS region for ARN generation |
| `ACCOUNT_ID` | `000000000000` | AWS account ID for ARN generation |
| `SQS_ENDPOINT` | `http://ess-queue-ess:9320` | SQS service endpoint for target delivery |
| `SNS_ENDPOINT` | `http://ess-enn-ess:9330` | SNS service endpoint for target delivery |

## Supported Operations

### Event Bus Operations

- `CreateEventBus` — create a named event bus
- `DescribeEventBus` — describe an event bus by name
- `ListEventBuses` — list all event buses (supports `Limit`, `NextToken`, `NamePrefix`)
- `DeleteEventBus` — delete a custom event bus (the `default` bus cannot be deleted)

### Rule Operations

- `PutRule` — create or update a rule on an event bus (requires `EventPattern` or `ScheduleExpression`)
- `DescribeRule` — describe a rule by name and bus
- `ListRules` — list rules on an event bus (supports `Limit`, `NextToken`, `NamePrefix`)
- `EnableRule` / `DisableRule` — toggle a rule's state
- `DeleteRule` — delete a rule (all targets must be removed first)

### Target Operations

- `PutTargets` — attach up to 5 targets to a rule (SQS, SNS, HTTP)
- `ListTargetsByRule` — list targets on a rule
- `RemoveTargets` — detach targets from a rule by ID

### Event Operations

- `PutEvents` — publish events to an event bus; matched rules deliver to targets asynchronously
- `TestEventPattern` — test whether an event matches a pattern without firing targets

### Tag Operations (stubs)

- `ListTagsForResource` — returns empty tag list
- `TagResource` / `UntagResource` — accepted but no-op

## Example Requests

### Create an event bus

```bash
curl -s http://localhost:9340/ \
  -H 'Content-Type: application/x-amz-json-1.1' \
  -H 'X-Amz-Target: AWSEvents.CreateEventBus' \
  -d '{"Name": "orders"}'
```

### Create a rule with an event pattern

```bash
curl -s http://localhost:9340/ \
  -H 'Content-Type: application/x-amz-json-1.1' \
  -H 'X-Amz-Target: AWSEvents.PutRule' \
  -d '{
    "Name": "order-placed",
    "EventBusName": "orders",
    "EventPattern": "{\"source\": [\"shop\"], \"detail-type\": [\"OrderPlaced\"]}",
    "State": "ENABLED"
  }'
```

### Attach an SQS target

```bash
curl -s http://localhost:9340/ \
  -H 'Content-Type: application/x-amz-json-1.1' \
  -H 'X-Amz-Target: AWSEvents.PutTargets' \
  -d '{
    "Rule": "order-placed",
    "EventBusName": "orders",
    "Targets": [
      {
        "Id": "sqs-target",
        "Arn": "arn:aws:sqs:us-east-1:000000000000:order-queue"
      }
    ]
  }'
```

### Publish an event

```bash
curl -s http://localhost:9340/ \
  -H 'Content-Type: application/x-amz-json-1.1' \
  -H 'X-Amz-Target: AWSEvents.PutEvents' \
  -d '{
    "Entries": [
      {
        "Source": "shop",
        "DetailType": "OrderPlaced",
        "Detail": "{\"orderId\": \"12345\", \"amount\": 99.99}",
        "EventBusName": "orders"
      }
    ]
  }'
```

### Test a pattern match

```bash
curl -s http://localhost:9340/ \
  -H 'Content-Type: application/x-amz-json-1.1' \
  -H 'X-Amz-Target: AWSEvents.TestEventPattern' \
  -d '{
    "EventPattern": "{\"source\": [\"shop\"]}",
    "Event": "{\"source\": \"shop\", \"detail-type\": \"OrderPlaced\", \"detail\": {}}"
  }'
```

### Create a scheduled rule

```bash
curl -s http://localhost:9340/ \
  -H 'Content-Type: application/x-amz-json-1.1' \
  -H 'X-Amz-Target: AWSEvents.PutRule' \
  -d '{
    "Name": "every-five-minutes",
    "ScheduleExpression": "rate(5 minutes)",
    "State": "ENABLED"
  }'
```

## Event Pattern Matching

Rules with an `EventPattern` use EventBridge pattern matching syntax. Supported pattern operators:

- **Exact match** — `{"source": ["myapp"]}`
- **Prefix** — `{"source": [{"prefix": "aws."}]}`
- **Anything-but** — `{"source": [{"anything-but": "internal"}]}`
- **Numeric** — `{"detail": {"price": [{"numeric": [">=", 10]}]}}`
- **Exists** — `{"detail": {"key": [{"exists": true}]}}`
- **Nested fields** — patterns can match at any depth

## Schedule Expressions

Rules with a `ScheduleExpression` fire on a background timer (1-minute resolution):

- **Rate** — `rate(5 minutes)`, `rate(1 hour)`, `rate(1 day)`
- **Cron** — `cron(0 12 * * ? *)` (6-field AWS cron format)

Scheduled events are delivered with `detail-type: "Scheduled Event"` and `source: "aws.events"`.

## Input Transformations

Targets support three input transformation modes:

| Field | Behavior |
|-------|----------|
| `Input` | Static string replaces the entire event payload |
| `InputPath` | JSONPath expression extracts a subset of the event |
| `InputTransformer` | Template-based transformation with `InputPathsMap` and `InputTemplate` |

## Admin Endpoints

- `GET /admin/api/summary` — event bus, rule, and target counts
- `GET /admin/api/resources` — detailed breakdown of buses, rules, and targets
- `GET /admin/api/activity` — paginated activity log (supports `maxResults` and `nextToken`)
- `GET /admin/api/export` — export all event buses, rules, and targets as JSON
- `POST /admin/api/import` — import event buses and rules from JSON

## Architecture

### Project Structure

```
drawbridge/
├── cmd/drawbridge/main.go          # Entry point and config loading
├── internal/
│   ├── server/server.go            # HTTP routing and API handlers
│   ├── model/types.go              # Request/response types
│   ├── store/store.go              # Thread-safe in-memory storage
│   ├── delivery/deliver.go         # Async target delivery (SQS/SNS/HTTP)
│   └── schedule/scheduler.go       # Background scheduler for rate/cron rules
├── Dockerfile
└── go.mod
```

### Storage

All data is stored in-memory using a thread-safe store (`sync.RWMutex`). The `default` event bus is created automatically on startup. Data does not persist across restarts.

### Target Delivery

Targets are delivered asynchronously after event pattern matching:

- **SQS targets** — form-encoded `SendMessage` via the SQS emulator
- **SNS targets** — form-encoded `Publish` via the SNS emulator
- **HTTP/HTTPS targets** — direct POST with JSON payload

Delivery failures are logged but do not affect other targets.

### Error Responses

Errors follow the AWS JSON protocol format:

```json
{
  "__type": "ResourceNotFoundException",
  "message": "Event bus 'missing' does not exist."
}
```

Common error types: `ValidationException`, `ResourceNotFoundException`, `ResourceAlreadyExistsException`, `InvalidEventPatternException`, `LimitExceededException`.

## Testing

```bash
cd services/drawbridge
go test ./...
```

Unit tests cover event bus CRUD, rule management, target operations, event processing, pattern matching, scheduled rules, admin endpoints, and error conditions.

## Notes

- This emulator intentionally prioritizes local dev compatibility over strict AWS parity.
- Tag operations are accepted but not persisted.
- Maximum of 5 targets per rule (matching the AWS default).
- Scheduled rules have 1-minute resolution.

## License

Licensed under the Apache License, Version 2.0. See the root [LICENSE](../../LICENSE) file for details.

## Trademark Notice

This project is not affiliated with, endorsed by, or sponsored by Amazon Web Services (AWS). Amazon EventBridge is a trademark of Amazon.com, Inc., or its affiliates.
