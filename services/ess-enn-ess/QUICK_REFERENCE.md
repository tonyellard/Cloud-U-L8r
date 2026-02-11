# Quick Reference: ess-enn-ess SNS Emulator

## Starting the Services

### Option 1: Docker Compose (Recommended)
```bash
cd /home/tony/Documents/cloud-u-l8r
docker-compose up -d
```
This starts all services including SNS, SQS, and admin dashboard.

### Option 2: Manual Start
```bash
# Terminal 1: Start SQS emulator
cd /home/tony/Documents/cloud-u-l8r/services/ess-queue-ess
docker-compose up

# Terminal 2: Build and start SNS emulator
cd /home/tony/Documents/cloud-u-l8r/services/ess-enn-ess
go build -o ess-enn-ess ./cmd/ess-enn-ess
./ess-enn-ess
```

## Accessing Services

| Service | Port | URL |
|---------|------|-----|
| SNS API | 9330 | http://localhost:9330 |
| Admin Dashboard | 9331 | http://localhost:9331 |
| SQS (ess-queue-ess) | 9320 | http://localhost:9320 |

## Key Endpoints

### SNS API (Port 9330)
- Health: `GET /health`
- Actions: `POST /` with form parameters

### Admin Dashboard (Port 9331)
- Dashboard UI: `GET /`
- Topics: `GET /api/topics`
- Subscriptions: `GET /api/subscriptions`
- Activities: `GET /api/activities`
- Statistics: `GET /api/stats`
- Export: `GET /api/export`

## Common AWS CLI Commands

### Setup
```bash
export AWS_ACCESS_KEY_ID=testing
export AWS_SECRET_ACCESS_KEY=testing
export AWS_DEFAULT_REGION=us-east-1
```

### Topics
```bash
# Create topic
aws --endpoint-url=http://localhost:9330 sns create-topic --name my-topic

# List topics
aws --endpoint-url=http://localhost:9330 sns list-topics

# Get topic attributes
aws --endpoint-url=http://localhost:9330 sns get-topic-attributes \
  --topic-arn arn:aws:sns:us-east-1:000000000000:my-topic

# Set topic attribute
aws --endpoint-url=http://localhost:9330 sns set-topic-attributes \
  --topic-arn arn:aws:sns:us-east-1:000000000000:my-topic \
  --attribute-name DisplayName \
  --attribute-value "My Topic"
```

### Subscriptions
```bash
# Subscribe to HTTP endpoint
aws --endpoint-url=http://localhost:9330 sns subscribe \
  --topic-arn arn:aws:sns:us-east-1:000000000000:my-topic \
  --protocol http \
  --notification-endpoint http://example.com/webhook

# Subscribe to SQS
aws --endpoint-url=http://localhost:9330 sns subscribe \
  --topic-arn arn:aws:sns:us-east-1:000000000000:my-topic \
  --protocol sqs \
  --notification-endpoint arn:aws:sqs:us-east-1:000000000000:my-queue

# List subscriptions
aws --endpoint-url=http://localhost:9330 sns list-subscriptions

# Unsubscribe
aws --endpoint-url=http://localhost:9330 sns unsubscribe \
  --subscription-arn arn:aws:sns:us-east-1:000000000000:my-topic:99999999-9999-9999-9999-999999999999
```

### Publishing
```bash
# Publish message
aws --endpoint-url=http://localhost:9330 sns publish \
  --topic-arn arn:aws:sns:us-east-1:000000000000:my-topic \
  --message "Hello, World!" \
  --subject "Test Message"
```

## Testing Scripts

```bash
cd /home/tony/Documents/cloud-u-l8r/services/ess-enn-ess

# Test message publishing
./test_publish.sh

# Test SQS integration
./test_sqs_integration.sh

# Quick test with auto-start
./test_sqs_quick.sh

# Test HTTP retry logic
./test_http_retry.sh

# Test admin dashboard
./test_admin_dashboard.sh
```

## Dashboard Features

### Tabs
1. **Topics** - View all SNS topics with subscription counts
2. **Subscriptions** - View all subscriptions with protocol and status
3. **Activity Log** - Real-time activity stream with filtering
4. **Export/Import** - Download configuration as YAML

### Statistics Cards (Auto-refresh every 3 seconds)
- Topics
- Subscriptions (with confirmed count)
- Messages Published
- Deliveries (with failed count)
- Total Events

### Filters
- Filter subscriptions by topic ARN
- Filter activities by topic, event type, or status

## Configuration

Edit `../../config/ess-enn-ess.config.yaml`:

```yaml
server:
  api_port: 9330       # SNS API port
  admin_port: 9331     # Admin dashboard port
  host: "0.0.0.0"

sqs:
  enabled: true
  endpoint: "http://ess-queue-ess:9320"  # SQS emulator endpoint

delivery:
  http:
    max_retries: 3
    backoff_initial_ms: 100
    backoff_max_ms: 10000
    backoff_multiplier: 2
```

## Development Commands

```bash
# Build
cd /home/tony/Documents/cloud-u-l8r/services/ess-enn-ess
go build -o ess-enn-ess ./cmd/ess-enn-ess

# Run tests
go test ./...

# Run with specific config
./ess-enn-ess -config config/custom.yaml

# View logs (if running in foreground)
# Logs include: topic operations, subscriptions, publish, delivery, retries

# Check server health
curl http://localhost:9330/health  # SNS API
curl http://localhost:9331/health  # Admin dashboard
```

## Troubleshooting

### Service won't start
```bash
# Check port availability
lsof -i :9330  # SNS API
lsof -i :9331  # Admin dashboard
lsof -i :9320  # SQS emulator

# Kill if necessary
kill -9 <PID>
```

### SQS delivery not working
1. Verify ess-queue-ess is running: `curl http://localhost:9320/health`
2. Check config has: `sqs.endpoint: "http://ess-queue-ess:9320"`
3. Review activity log for delivery errors

### Dashboard not loading
1. Verify admin server running: `curl http://localhost:9331/health`
2. Check browser console for API errors
3. Verify SNS server is running: `curl http://localhost:9330/health`

### Messages not delievering
1. Check subscription status in dashboard
2. Review activity log tab for delivery errors
3. For HTTP: verify endpoint is accessible
4. For SQS: verify queue exists in ess-queue-ess

## Project Structure

```
ess-enn-ess/
├── cmd/ess-enn-ess/        # Entry point
├── internal/
│   ├── server/             # HTTP server & handlers
│   ├── topic/              # Topic store
│   ├── subscription/       # Subscriptions
│   ├── message/            # Message formatting
│   ├── delivery/           # Delivery with retries
│   ├── activity/           # Activity logging
│   ├── admin/              # Admin dashboard
│   │   ├── admin.go        # Backend API
│   │   └── dashboard.go    # HTML UI
│   └── config/             # Configuration
├── config/
│   └── config.example.yaml # Example config (central config lives in ../../config)
├── test_*.sh               # Test scripts
└── README.md               # Documentation
```

## Useful Curl Examples

```bash
# Create topic
curl -X POST http://localhost:9330/ \
  -d "Action=CreateTopic&Name=my-topic"

# Publish message
curl -X POST http://localhost:9330/ \
  -d "Action=Publish" \
  -d "TopicArn=arn:aws:sns:us-east-1:000000000000:my-topic" \
  -d "Message=Hello"

# Get all topics (admin API)
curl http://localhost:9331/api/topics | jq

# Get statistics
curl http://localhost:9331/api/stats | jq

# Get activity log
curl http://localhost:9331/api/activities?limit=50 | jq

# Download YAML export
curl http://localhost:9331/api/export > backup.yaml
```

## Implementation Status

- ✅ Phase 1: Core topic management
- ✅ Phase 3: Subscription management
- ✅ Phase 4: Message publishing
- ✅ Phase 5: SQS integration
- ✅ Phase 6: HTTP retry logic
- ✅ Phase 7: Admin dashboard
- 🔄 Phase 8+: Advanced features (FIFO, DLQ, etc.)

## Support

For issues or questions, check:
1. [README.md](./README.md) - Full documentation
2. [RETRY_LOGIC.md](./RETRY_LOGIC.md) - Delivery retry details
3. [PHASE7_DASHBOARD.md](./PHASE7_DASHBOARD.md) - Dashboard implementation details
4. Activity log in admin dashboard for error details
