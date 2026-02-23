# EventBridge Emulator: Implementation Plan

## Service Name: **drawbridge**

The naming convention uses "close but not quite" puns on AWS service names:
- essthree = S3 (phonetic)
- cloudfauxnt = CloudFront ("faux" = fake embedded)
- ess-queue-ess = SQS (phonetic, "queue" spelled out)
- ess-enn-ess = SNS (phonetic)
- kay-vee = KV (phonetic abbreviation)

**drawbridge** swaps "EventBridge" for a specific type of bridge that can be
raised and lowered — a fun visual for an emulator that can be started/stopped.
It's recognizably a bridge but clearly not the right one.

**Port: 9340** (fills the gap between ess-enn-ess at 9330 and kay-vee at 9350)

---

## Wire Protocol

EventBridge uses the same JSON protocol as kay-vee (SSM/Secrets Manager):
- `POST /` with `Content-Type: application/x-amz-json-1.1`
- Action dispatched via `X-Amz-Target: AWSEvents.<Action>`
- JSON request/response bodies
- Errors as `{"__type": "ErrorType", "message": "..."}`

This means the handler dispatch pattern from `kay-vee/internal/server/router.go`
can be reused almost directly.

---

## Scope: MVP Actions (15 actions)

### Event Buses (4)
| Action | Description |
|--------|-------------|
| `CreateEventBus` | Create custom event bus (+ always have "default") |
| `DescribeEventBus` | Get event bus details |
| `ListEventBuses` | List all event buses |
| `DeleteEventBus` | Delete custom event bus (cannot delete "default") |

### Rules (6)
| Action | Description |
|--------|-------------|
| `PutRule` | Create or update a rule with event pattern and/or schedule |
| `DescribeRule` | Get rule details |
| `ListRules` | List rules on an event bus |
| `DeleteRule` | Delete a rule (must remove targets first) |
| `EnableRule` | Enable a disabled rule |
| `DisableRule` | Disable a rule |

### Targets (3)
| Action | Description |
|--------|-------------|
| `PutTargets` | Add/update targets on a rule (up to 5 per rule) |
| `ListTargetsByRule` | List targets for a rule |
| `RemoveTargets` | Remove targets from a rule |

### Events (2)
| Action | Description |
|--------|-------------|
| `PutEvents` | Publish events (up to 10 per call), match rules, deliver to targets |
| `TestEventPattern` | Test if an event matches a pattern without side effects |

### Deferred to later (not in MVP)
- Archives/Replays, API Destinations, Connections, Global Endpoints, Partner Events, Tagging, Permissions, Schedules

---

## Data Model

### Event Bus
```go
type EventBus struct {
    Name           string
    Arn            string
    CreationTime   time.Time
    Rules          map[string]*Rule  // keyed by rule name
}
```
A "default" bus is created at startup.

### Rule
```go
type Rule struct {
    Name               string
    Arn                string
    EventBusName       string
    EventPattern       string    // JSON pattern string
    ScheduleExpression string    // "rate(...)" or "cron(...)" — parsed but schedules not executed in MVP
    State              string    // "ENABLED" or "DISABLED"
    Description        string
    Targets            map[string]*Target  // keyed by target ID
    CreatedBy          string
}
```

### Target
```go
type Target struct {
    Id       string
    Arn      string
    Input    string  // static replacement input (optional)
    InputPath string // JSONPath to extract subset (optional)
    InputTransformer *InputTransformer // template-based (optional)
    SqsParameters    *SqsParameters   // for FIFO queue MessageGroupId
}

type InputTransformer struct {
    InputPathsMap map[string]string
    InputTemplate string
}

type SqsParameters struct {
    MessageGroupId string
}
```

### Event (internal, for matching + delivery)
```go
type Event struct {
    Version    string    // always "0"
    ID         string    // UUID
    Source     string
    Account    string
    Time       time.Time
    Region     string
    Resources  []string
    DetailType string
    Detail     json.RawMessage
}
```

---

## Event Pattern Matching Engine

This is the most complex component. The pattern matcher evaluates a JSON pattern
against a JSON event. Implementation plan:

### Phase 1 — Core matching (MVP)
- Exact string matching: `{"source": ["my.app"]}`
- Multiple values (OR): `{"detail-type": ["A", "B"]}`
- Nested field matching: `{"detail": {"status": ["ok"]}}`
- Fields not in pattern are wildcards

### Phase 2 — Comparison operators (MVP)
- `prefix`: `[{"prefix": "prod-"}]`
- `exists`: `[{"exists": true}]` / `[{"exists": false}]`
- `numeric`: `[{"numeric": [">", 100]}]` and range `[{"numeric": [">", 0, "<=", 5]}]`
- `anything-but`: `[{"anything-but": "test"}]` and `[{"anything-but": ["a","b"]}]`

### Phase 3 — Extended operators (post-MVP)
- `suffix`, `wildcard`, `equals-ignore-case`, `cidr`
- `anything-but` with prefix/suffix/wildcard variants
- `$or` cross-field logic

The pattern matching engine should be a standalone internal package
(`internal/matching/`) with thorough unit tests, since it's the core
differentiator of EventBridge.

---

## Target Delivery

When `PutEvents` is called:
1. For each event, iterate all rules on the target event bus
2. Skip rules with `State: "DISABLED"`
3. Parse the rule's `EventPattern` and match against the event
4. For matching rules, deliver the event to each target

### Supported target types (MVP)

| Target | Delivery mechanism | Endpoint |
|--------|-------------------|----------|
| **SQS** | HTTP POST to ess-queue-ess with `Action=SendMessage` form-encoded | `http://ess-queue-ess:9320` |
| **SNS** | HTTP POST to ess-enn-ess with `Action=Publish` form-encoded | `http://ess-enn-ess:9330` |
| **HTTP/HTTPS** | HTTP POST with event JSON body | user-specified URL |
| **CloudWatch Logs** | Log the event (write to slog, no external call) | N/A |

Target ARNs will be parsed to determine the target type:
- `arn:aws:sqs:*:*:queue-name` → SQS delivery
- `arn:aws:sns:*:*:topic-name` → SNS delivery
- `http://` or `https://` → HTTP delivery
- Others → log a warning

### Input transformation
- If `Input` is set: send the static string as the message body
- If `InputPath` is set: extract the subtree from the event JSON
- If `InputTransformer` is set: apply template substitution
- If none: send the full event JSON

### Delivery is synchronous within PutEvents
- No background workers in MVP
- Delivery failures are reported in the `PutEvents` response `FailedEntryCount` + `Entries[].ErrorCode`/`ErrorMessage`

---

## Directory Structure

```
services/drawbridge/
├── cmd/
│   └── drawbridge/
│       └── main.go
├── internal/
│   ├── server/
│   │   ├── server.go           # Server struct, chi router, middleware
│   │   ├── handlers.go         # AWS API action handlers
│   │   ├── admin.go            # Admin API endpoints
│   │   └── handlers_test.go    # Unit tests
│   ├── matching/
│   │   ├── matcher.go          # Event pattern matching engine
│   │   └── matcher_test.go     # Exhaustive matching tests
│   ├── delivery/
│   │   ├── delivery.go         # Target delivery logic (SQS, SNS, HTTP)
│   │   └── delivery_test.go
│   └── store/
│       ├── store.go            # In-memory storage (buses, rules, targets)
│       └── store_test.go
├── Dockerfile
├── go.mod
├── go.sum
├── LICENSE
└── NOTICE
```

---

## Implementation Steps

### Step 1: Scaffold the service
- Create directory structure
- `go.mod` with dependencies: `chi`, `google/uuid`, `pkg/health`, `pkg/awserrors`, `pkg/activity`
- `main.go` with flag parsing, config loading, HTTP server startup
- `Dockerfile` following the multi-stage pattern
- Add to `go.work`, `docker-compose.yml`, `Makefile`

### Step 2: In-memory store
- `store.go`: Thread-safe store with `sync.RWMutex`
- `NewStore()` creates a "default" event bus
- CRUD operations for event buses, rules, targets
- Unit tests

### Step 3: Pattern matching engine
- `matcher.go`: `Match(pattern string, event json.RawMessage) (bool, error)`
- Phase 1 operators: exact match, multi-value OR, nested fields
- Phase 2 operators: prefix, exists, numeric, anything-but
- Exhaustive unit tests (this is the critical component)

### Step 4: AWS API handlers
- `server.go`: chi router, `X-Amz-Target` dispatch, activity middleware
- `handlers.go`: All 15 MVP actions
- `TestEventPattern` exercises the matching engine
- Unit tests for each action

### Step 5: Target delivery
- `delivery.go`: Deliver events to SQS, SNS, HTTP targets
- Input transformation (Input, InputPath, InputTransformer)
- Wire into `PutEvents` handler
- Unit tests with httptest mock targets

### Step 6: Admin interface
- `admin.go`: Admin API endpoints
  - `GET /admin/api/summary` — bus/rule/target counts
  - `GET /admin/api/buses` — list event buses with rules
  - `GET /admin/api/activity` — activity log
  - `GET /admin/api/export` — JSON export
  - `POST /admin/api/import` — JSON import

### Step 7: Configuration via Terraform (not YAML)
- No `config/drawbridge.config.yaml` — starting with this service, Terraform
  is the config method of choice
- The service boots with an empty "default" event bus and no pre-seeded rules
- All configuration (buses, rules, targets) is applied via the AWS EventBridge
  Terraform provider pointing at `http://localhost:9340`
- Service endpoint is configurable via environment variables:
  - `PORT` (default 9340)
  - `REGION` (default us-east-1)
  - `ACCOUNT_ID` (default 000000000000)
  - `SQS_ENDPOINT` (default http://ess-queue-ess:9320)
  - `SNS_ENDPOINT` (default http://ess-enn-ess:9330)

### Step 8: Admin console integration
- Add proxy routes in admin-console for drawbridge
- Add drawbridge tab/section in `web/app.js`
- Show event buses, rules, targets, activity log

### Step 9: Integration tests
- Test PutEvents → rule match → SQS delivery end-to-end
- Test PutEvents → rule match → SNS delivery end-to-end
- Add to `tests/integration/test_cross_service.sh`

### Step 10: Documentation
- Update `CLAUDE.md` with new service details
