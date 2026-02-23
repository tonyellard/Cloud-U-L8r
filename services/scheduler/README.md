# scheduler - Local EventBridge Scheduler Emulator

`scheduler` is a local emulator for AWS EventBridge Scheduler, supporting schedule groups, schedules with rate/cron/at expressions, and automatic delivery to SQS and SNS targets. Part of the cloud-u-l8r emulator suite.

## Features

- AWS-style JSON RPC over `X-Amz-Target` (`AWSScheduler.*` actions)
- Schedule groups with a pre-created `default` group
- Rate, cron, and one-time `at()` schedule expressions
- Automatic target delivery to SQS and SNS endpoints
- Flexible time windows and start/end date constraints
- Action-after-completion support (auto-delete for one-time schedules)
- In-memory storage for fast local development
- Activity logging for all API calls

## Quick Start

### Local

```bash
cd services/scheduler
go run ./cmd/scheduler
```

Service default endpoint: `http://localhost:9342`

Health check:

```bash
curl http://localhost:9342/health
```

### Docker

```bash
docker build -t scheduler .
docker run --rm -p 9342:9342 scheduler
```

## Configuration

Scheduler is configured via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `9342` | Server listen port |
| `REGION` | `us-east-1` | AWS region for ARN generation |
| `ACCOUNT_ID` | `000000000000` | AWS account ID for ARN generation |
| `SQS_ENDPOINT` | `http://ess-queue-ess:9320` | SQS service endpoint for target delivery |
| `SNS_ENDPOINT` | `http://ess-enn-ess:9330` | SNS service endpoint for target delivery |

## Supported Operations

### Schedule Group Operations

- `CreateScheduleGroup` — create a named schedule group
- `GetScheduleGroup` — describe a schedule group by name
- `ListScheduleGroups` — list all groups (supports `MaxResults`, `NextToken`, `NamePrefix`)
- `DeleteScheduleGroup` — delete a group and all its schedules (the `default` group cannot be deleted)

### Schedule Operations

- `CreateSchedule` — create a schedule with a target and expression
- `GetSchedule` — describe a schedule by name and group
- `UpdateSchedule` — modify an existing schedule's expression, target, or state
- `ListSchedules` — list schedules in a group (supports `MaxResults`, `NextToken`, `NamePrefix`, `State` filter)
- `DeleteSchedule` — delete a schedule from a group

## Example Requests

### Create a schedule group

```bash
curl -s http://localhost:9342/ \
  -H 'Content-Type: application/x-amz-json-1.1' \
  -H 'X-Amz-Target: AWSScheduler.CreateScheduleGroup' \
  -d '{"Name": "batch-jobs"}'
```

### Create a rate-based schedule

```bash
curl -s http://localhost:9342/ \
  -H 'Content-Type: application/x-amz-json-1.1' \
  -H 'X-Amz-Target: AWSScheduler.CreateSchedule' \
  -d '{
    "Name": "poll-every-5min",
    "GroupName": "batch-jobs",
    "ScheduleExpression": "rate(5 minutes)",
    "State": "ENABLED",
    "FlexibleTimeWindow": {"Mode": "OFF"},
    "Target": {
      "Arn": "arn:aws:sqs:us-east-1:000000000000:work-queue",
      "RoleArn": "arn:aws:iam::000000000000:role/scheduler-role",
      "Input": "{\"action\": \"poll\"}"
    }
  }'
```

### Create a cron-based schedule

```bash
curl -s http://localhost:9342/ \
  -H 'Content-Type: application/x-amz-json-1.1' \
  -H 'X-Amz-Target: AWSScheduler.CreateSchedule' \
  -d '{
    "Name": "weekday-report",
    "ScheduleExpression": "cron(0 9 ? * MON-FRI *)",
    "State": "ENABLED",
    "FlexibleTimeWindow": {"Mode": "OFF"},
    "Target": {
      "Arn": "arn:aws:sns:us-east-1:000000000000:report-topic",
      "RoleArn": "arn:aws:iam::000000000000:role/scheduler-role",
      "Input": "{\"report\": \"daily-summary\"}"
    }
  }'
```

### Create a one-time schedule

```bash
curl -s http://localhost:9342/ \
  -H 'Content-Type: application/x-amz-json-1.1' \
  -H 'X-Amz-Target: AWSScheduler.CreateSchedule' \
  -d '{
    "Name": "deploy-cutover",
    "ScheduleExpression": "at(2026-03-01T03:00:00)",
    "ActionAfterCompletion": "DELETE",
    "FlexibleTimeWindow": {"Mode": "OFF"},
    "Target": {
      "Arn": "arn:aws:sqs:us-east-1:000000000000:deploy-queue",
      "RoleArn": "arn:aws:iam::000000000000:role/scheduler-role"
    }
  }'
```

### List schedules in a group

```bash
curl -s http://localhost:9342/ \
  -H 'Content-Type: application/x-amz-json-1.1' \
  -H 'X-Amz-Target: AWSScheduler.ListSchedules' \
  -d '{"GroupName": "batch-jobs"}'
```

## Schedule Expressions

The background runner evaluates schedules every 1 minute and delivers to targets automatically.

### Rate Expressions

Periodic schedules: `rate(N unit)` where unit is `minute(s)`, `hour(s)`, or `day(s)`.

```
rate(1 minute)     # every minute
rate(5 minutes)    # every 5 minutes
rate(1 hour)       # every hour
rate(7 days)       # every 7 days
```

Singular/plural must match the value (e.g., `rate(1 minute)` not `rate(1 minutes)`).

### Cron Expressions

Calendar-based schedules using 6-field AWS cron format: `cron(min hour dom month dow year)`.

```
cron(0 12 * * ? *)          # every day at noon UTC
cron(0 9 ? * MON-FRI *)     # weekdays at 9 AM UTC
cron(0 18 ? * MON,WED,FRI *)  # Mon/Wed/Fri at 6 PM UTC
cron(*/5 * * * ? *)         # every 5 minutes
```

Supported field features: ranges (`9-17`), lists (`MON,WED,FRI`), wildcards (`*`), step values (`*/5`), and the `?` specifier for day-of-month or day-of-week.

### At Expressions

One-time schedules: `at(yyyy-mm-ddThh:mm:ss)`.

```
at(2026-03-01T03:00:00)    # fire once at this UTC time
```

When paired with `"ActionAfterCompletion": "DELETE"`, the schedule is automatically removed after firing.

## Target Delivery

When a schedule fires, the runner delivers to the configured target:

- **SQS targets** — form-encoded `SendMessage` via the SQS emulator (supports `MessageGroupId` for FIFO queues via `SqsParameters`)
- **SNS targets** — form-encoded `Publish` via the SNS emulator

If the target's `Input` field is empty, a default payload is sent:

```json
{
  "source": "aws.scheduler",
  "detail-type": "Scheduled Event",
  "detail": {}
}
```

## Admin Endpoints

- `GET /admin/api/summary` — schedule group and schedule counts
- `GET /admin/api/resources` — detailed groups with their schedules
- `GET /admin/api/activity` — paginated activity log (supports `maxResults` and `nextToken`)

## Architecture

### Project Structure

```
scheduler/
├── cmd/scheduler/main.go           # Entry point and config loading
├── internal/
│   ├── server/server.go            # HTTP routing and API handlers
│   ├── model/types.go              # Request/response types
│   ├── store/store.go              # Thread-safe in-memory storage
│   ├── runner/runner.go            # Background schedule runner (1-min tick)
│   └── delivery/deliver.go         # Target delivery (SQS/SNS)
├── Dockerfile
└── go.mod
```

### Storage

All data is stored in-memory using a thread-safe store (`sync.RWMutex`). The `default` schedule group is created automatically on startup. Data does not persist across restarts.

### Background Runner

The runner goroutine ticks every 1 minute and evaluates all enabled schedules:

- **Rate schedules** fire based on elapsed time since the last fire
- **Cron schedules** fire when the current minute matches the expression
- **At schedules** fire once when the scheduled time arrives

Fire deduplication prevents the same schedule from firing multiple times in the same minute.

### Error Responses

Errors follow the AWS JSON protocol format:

```json
{
  "__type": "ValidationException",
  "message": "ScheduleExpression is not valid."
}
```

Common error types: `ValidationException`, `ResourceNotFoundException`, `ConflictException`.

## Testing

```bash
cd services/scheduler
go test ./...
```

Unit tests cover schedule group CRUD, schedule CRUD, expression validation, state filtering, admin endpoints, and error conditions.

## Notes

- This emulator intentionally prioritizes local dev compatibility over strict AWS parity.
- Schedules have 1-minute resolution (the runner ticks every 60 seconds).
- The `default` schedule group cannot be deleted.
- `ScheduleExpressionTimezone` is accepted but not enforced; all times are UTC.
- `FlexibleTimeWindow` is stored but not enforced during execution.

## License

Licensed under the Apache License, Version 2.0. See the root [LICENSE](../../LICENSE) file for details.

## Trademark Notice

This project is not affiliated with, endorsed by, or sponsored by Amazon Web Services (AWS). Amazon EventBridge Scheduler is a trademark of Amazon.com, Inc., or its affiliates.
