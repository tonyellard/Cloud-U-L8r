# HTTP Delivery Retry Logic - Phase 6

## Overview

Phase 6 implements intelligent retry logic for HTTP/HTTPS endpoint deliveries with exponential backoff, error categorization, and comprehensive logging.

## Features

### 1. Automatic Retry with Exponential Backoff

When an HTTP delivery fails with a retryable error, the system automatically retries with exponentially increasing delays:

- **Base backoff**: Configurable via `http.retry_backoff_ms` (default: 100ms)
- **Exponential increase**: Each retry doubles the delay (2^attempt)
- **Maximum backoff**: Capped at 30 seconds to prevent excessive delays
- **Configurable retries**: Set via `http.max_retries` (default: 3)

**Example retry timeline with 100ms base:**
```
Attempt 1: Immediate
Attempt 2: Wait 100ms  (100 * 2^0)
Attempt 3: Wait 200ms  (100 * 2^1)
Attempt 4: Wait 400ms  (100 * 2^2)
```

### 2. Error Categorization

The system intelligently categorizes errors to determine retry behavior:

#### Transient Errors (Will Retry)
- **5xx Server Errors**: 500, 502, 503, 504, etc.
  - Server temporarily unavailable
  - Indicates endpoint issues that may resolve
- **429 Too Many Requests**
  - Rate limiting - retry after backoff
- **408 Request Timeout**
  - Temporary network/timing issue
- **Network Errors**: Connection refused, timeouts, DNS failures
  - Infrastructure issues that may resolve

#### Permanent Errors (Will NOT Retry)
- **4xx Client Errors** (except 408, 429): 400, 401, 403, 404, etc.
  - Bad request format
  - Authentication/authorization failures
  - Not found errors
  - These won't resolve with retries

### 3. Enhanced Logging

Each delivery attempt is logged with detailed information:

```
Initial attempt:
  - message_id
  - endpoint
  - protocol
  
Retry attempts:
  - attempt number (e.g., "retry attempt 2/3")
  - backoff delay
  - retry status in activity log
  
Final result:
  - success or failure
  - total attempts made
  - final error message
```

### 4. Activity Log Integration

All delivery attempts are tracked in the activity logger:
- **StatusSuccess**: Delivery succeeded
- **StatusRetrying**: Retry in progress
- **StatusFailed**: Final failure after all retries

## Configuration

### config.yaml

```yaml
http:
  # Enable/disable HTTP endpoint subscriptions
  enabled: true
  
  # Maximum number of delivery retry attempts
  # 0 = no retries, just one attempt
  # 3 = one initial attempt + 3 retries (4 total attempts)
  max_retries: 3
  
  # Base backoff time in milliseconds between retries
  # Actual delay = retry_backoff_ms * (2 ^ attempt)
  retry_backoff_ms: 100
  
  # HTTP client timeout in seconds
  timeout_seconds: 5
```

## Usage Examples

### Example 1: Successful Delivery (No Retries)

```bash
curl -X POST http://localhost:9330/ \
  -d "Action=Publish" \
  -d "TopicArn=arn:aws:sns:us-east-1:123456789012:my-topic" \
  -d "Message=Hello World"
```

**Behavior**: Single delivery attempt succeeds → StatusSuccess logged

### Example 2: Transient Error with Recovery

Endpoint returns 503, then 503, then 200:

```
Attempt 1 (0ms):    503 → Retry scheduled
Attempt 2 (+100ms): 503 → Retry scheduled  
Attempt 3 (+200ms): 200 → Success!
```

**Activity Log**:
1. StatusRetrying - "retry attempt 1/3"
2. StatusRetrying - "retry attempt 2/3"
3. StatusSuccess - "succeeded after retries"

### Example 3: Permanent Error

Endpoint returns 404:

```
Attempt 1: 404 → Permanent error, no retry
```

**Activity Log**:
- StatusFailed - "permanent error, not retrying"

### Example 4: Exhausted Retries

Endpoint always returns 503:

```
Attempt 1 (0ms):    503 → Retry
Attempt 2 (+100ms): 503 → Retry
Attempt 3 (+200ms): 503 → Retry
Attempt 4 (+400ms): 503 → All retries exhausted
```

**Activity Log**:
- Multiple StatusRetrying entries
- Final StatusFailed - "delivery failed after 4 attempts"

## Error Types

### TransientError

Represents temporary failures that should be retried:

```go
type TransientError struct {
    Err     error
    Message string
}
```

**Causes**:
- Network errors
- 5xx status codes
- 429 Too Many Requests
- 408 Request Timeout

### PermanentError

Represents permanent failures that should not be retried:

```go
type PermanentError struct {
    Err     error
    Message string
}
```

**Causes**:
- 4xx status codes (except 408, 429)
- Invalid endpoint URL
- Serialization errors

## Monitoring & Debugging

### View Retry Attempts

Check the SNS activity log:
```bash
curl http://localhost:9331/api/activity | jq '.[] | select(.event_type == "delivery")'
```

### View Logs

```bash
# Application logs show detailed retry information
tail -f /var/log/ess-enn-ess.log

# Look for:
# - "retrying HTTP delivery"
# - "HTTP delivery succeeded after retries"
# - "HTTP delivery failed after all retries"
```

### Admin Dashboard

The admin UI at `http://localhost:9331` shows real-time delivery attempts with status.

## Testing

Run the comprehensive retry test:

```bash
cd services/ess-enn-ess
./test_http_retry.sh
```

This test simulates:
1. Successful delivery (no retries)
2. Transient errors with eventual success
3. Permanent errors (no retries)

## Performance Considerations

### Retry Impact

With default settings (3 retries, 100ms base):
- **Worst case total time**: ~800ms (100 + 200 + 400)
- **Deliveries are asynchronous**: Don't block publish response
- **Concurrent deliveries**: Each subscription gets its own goroutine

### Tuning Recommendations

**High-volume, low-latency**:
```yaml
max_retries: 1
retry_backoff_ms: 50
```

**Reliability-focused**:
```yaml
max_retries: 5
retry_backoff_ms: 200
```

**No retries (fastest, least reliable)**:
```yaml
max_retries: 0
```

## Future Enhancements

Potential additions for future phases:
- Dead Letter Queue (DLQ) for messages that fail all retries
- Configurable retry strategies (linear, exponential with jitter)
- Circuit breaker pattern to prevent overwhelming failing endpoints
- Retry queue for background processing of failed deliveries
- Metric tracking for retry rates and success ratios

## Related Files

- Implementation: `internal/delivery/delivery.go`
- Configuration: `internal/config/config.go`
- Test: `test_http_retry.sh`
- Documentation: `README.md`
