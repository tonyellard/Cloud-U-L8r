# FIFO and DLQ Implementation Summary

## What Was Implemented

### 1. FIFO Queues (First-In-First-Out)
- **Queue Naming**: Queues ending in `.fifo` are automatically recognized as FIFO queues
- **Message Ordering**: Messages within the same `MessageGroupId` are processed in order
- **Message Deduplication**:
  - Content-based deduplication (MD5 hash of message body)
  - Explicit deduplication via `MessageDeduplicationId`
  - 5-minute deduplication window
- **Sequence Numbers**: Each message receives a unique, incrementing sequence number
- **Message Groups**: Parallel processing across different groups while maintaining order within groups

### 2. Dead Letter Queues (DLQ)
- **Automatic Message Movement**: Messages exceeding `maxReceiveCount` automatically move to DLQ
- **RedrivePolicy Configuration**: Configure DLQ target ARN and max receive count per queue
- **RedriveAllowPolicy**: Control which queues can use a given queue as their DLQ
- **Receive Count Tracking**: Each message tracks how many times it's been received

### 3. Message Redrive
- **StartMessageMoveTask**: Move messages from DLQ back to source queue
- **ListMessageMoveTasks**: List active redrive tasks (returns empty - tasks complete immediately)
- **CancelMessageMoveTask**: Cancel in-progress tasks (no-op since tasks are immediate)

## Code Changes

### queue.go
**New Fields in Message struct:**
- `MessageDeduplicationId string` - For FIFO deduplication
- `MessageGroupId string` - For FIFO ordering
- `SequenceNumber string` - Message ordering identifier

**New Fields in Queue struct:**
- `FifoQueue bool` - Is this a FIFO queue?
- `ContentBasedDeduplication bool` - Auto-deduplicate based on content?
- `deduplicationCache map[string]time.Time` - Track deduplicated messages
- `sequenceNumber int64` - Auto-incrementing sequence
- `RedrivePolicy *RedrivePolicy` - DLQ configuration
- `RedriveAllowPolicy *RedriveAllowPolicy` - DLQ access control

**New Structs:**
```go
type RedrivePolicy struct {
    DeadLetterTargetArn string
    MaxReceiveCount     int
}

type RedriveAllowPolicy struct {
    RedrivePermission string   // allowAll, denyAll, byQueue
    SourceQueueArns   []string
}
```

**Updated Functions:**
- `CreateQueue()` - Parse FIFO and RedrivePolicy attributes
- `SendMessage()` - Added deduplication and message group support
- `ReceiveMessages()` - Implemented FIFO ordering and DLQ auto-movement
- `moveToDLQ()` - New function to move messages to DLQ
- `RedriveMessages()` - New function to move messages back from DLQ

**New Helper Functions:**
- `parseRedrivePolicy()` - Parse JSON redrive policy string
- `parseRedriveAllowPolicy()` - Parse JSON redrive allow policy
- `findJSONValue()` - Simple JSON value extraction
- `extractQueueNameFromArn()` - Extract queue name from ARN

### handlers.go
**Updated Handlers:**
- `handleSendMessage()` - Accept FIFO parameters (MessageDeduplicationId, MessageGroupId)
- Added sequence number to response

**New Handlers:**
- `handleStartMessageMoveTask()` - Initiate message redrive
- `handleListMessageMoveTasks()` - List redrive tasks
- `handleCancelMessageMoveTask()` - Cancel redrive tasks

**Updated Router:**
Added cases for:
- `StartMessageMoveTask`
- `ListMessageMoveTasks`
- `CancelMessageMoveTask`

## Testing

### Python Test Suite (test/fifo_dlq_test.py)
Three comprehensive test functions:
1. **test_fifo_queue()**: Tests FIFO creation, message groups, deduplication, ordering
2. **test_dlq_and_redrive()**: Tests DLQ movement, receive count tracking
3. **test_fifo_with_dlq()**: Tests combining FIFO with DLQ functionality

### .NET Example (dotnet-fifo-example/)
Complete example demonstrating:
- FIFO queue creation and configuration
- Message group usage
- Content-based deduplication
- DLQ setup with redrive policy
- Failed message processing simulation
- DLQ message inspection

## How It Works

### FIFO Message Flow
```
1. Client sends message with MessageGroupId
2. Queue checks deduplication cache
3. If duplicate (within 5 min), return existing message
4. Generate sequence number
5. Store message in queue
6. On receive, group messages by MessageGroupId
7. Return one message per group (maintains order)
```

### DLQ Flow
```
1. Client receives message from main queue
2. ReceiveCount increments
3. If ReceiveCount >= maxReceiveCount:
   a. Remove message from main queue
   b. Reset message state
   c. Add to DLQ
4. Client can inspect DLQ
5. Fix issue causing failures
6. Use StartMessageMoveTask to redrive
```

### Deduplication Logic
```
Content-Based (if enabled):
- MD5 hash of message body = deduplication ID
- Check cache for hash
- If exists and < 5 min old, return existing message

Explicit:
- Client provides MessageDeduplicationId
- Same cache check logic
```

## API Examples

### Create FIFO Queue
```python
sqs.create_queue(
    QueueName='my-queue.fifo',
    Attributes={
        'FifoQueue': 'true',
        'ContentBasedDeduplication': 'true'
    }
)
```

### Send to FIFO Queue
```python
sqs.send_message(
    QueueUrl=queue_url,
    MessageBody='Order 123',
    MessageGroupId='orders',
    MessageDeduplicationId='order-123-v1'  # Optional
)
```

### Create Queue with DLQ
```python
import json

redrive_policy = {
    'deadLetterTargetArn': 'arn:aws:sqs:us-east-1:000000000000:my-dlq',
    'maxReceiveCount': 3
}

sqs.create_queue(
    QueueName='main-queue',
    Attributes={
        'RedrivePolicy': json.dumps(redrive_policy)
    }
)
```

### Redrive Messages
```python
sqs.start_message_move_task(
    SourceArn='arn:aws:sqs:us-east-1:000000000000:my-dlq',
    DestinationArn='arn:aws:sqs:us-east-1:000000000000:main-queue'
)
```

## Compatibility

### Tested With:
- ✅ AWS CLI v2 (JSON protocol)
- ✅ Python boto3
- ✅ .NET AWS SDK (AWSSDK.SQS)
- ✅ Query protocol (form-encoded)

### SQS API Operations Now Supported:
1. CreateQueue
2. DeleteQueue
3. ListQueues
4. SendMessage (with FIFO params)
5. ReceiveMessage (with FIFO ordering)
6. DeleteMessage
7. GetQueueAttributes
8. PurgeQueue
9. StartMessageMoveTask (NEW)
10. ListMessageMoveTasks (NEW)
11. CancelMessageMoveTask (NEW)

## Performance Characteristics

### FIFO Queues
- **Deduplication Check**: O(1) hash map lookup
- **Message Grouping**: O(n) where n = messages in queue
- **Ordering**: Natural insertion order maintained

### DLQ Operations
- **Move to DLQ**: O(n) where n = queue length (linear search for message)
- **Redrive**: O(m) where m = messages to move
- **Immediate Execution**: No async processing (completes synchronously)

## Future Enhancements (Not Implemented)

1. **Persistent Storage**: Save messages to disk
2. **Batch Operations**: SendMessageBatch, DeleteMessageBatch
3. **Long Polling**: Full waitTimeSeconds implementation
4. **Message Timers**: ChangeMessageVisibility
5. **Enhanced Metrics**: CloudWatch-style metrics
6. **Async Redrive**: Background task processing
7. **Configurable Deduplication Window**: Beyond 5 minutes

## Documentation

- **[docs/FIFO_AND_DLQ.md](../docs/FIFO_AND_DLQ.md)**: Complete user guide with examples
- **[test/fifo_dlq_test.py](../test/fifo_dlq_test.py)**: Python test suite
- **[dotnet-fifo-example/](../dotnet-fifo-example/)**: .NET example application

## Validation

All tests passing:
```bash
$ python3 test/fifo_dlq_test.py
✓ FIFO queue creation
✓ Message groups
✓ Content-based deduplication
✓ Sequence numbers
✓ DLQ creation
✓ Automatic message movement to DLQ
✓ Message redrive

$ cd dotnet-fifo-example && dotnet run
✓ All .NET SDK tests passed
```

## Key Design Decisions

1. **In-Memory Only**: Keeps implementation simple and fast
2. **Immediate Redrive**: No background processing complexity
3. **Fixed 5-Min Deduplication**: Matches AWS SQS behavior
4. **Group-Based FIFO**: One message per group allows parallel processing
5. **Auto-Detection**: .fifo suffix automatically enables FIFO mode
6. **Simple JSON Parsing**: Custom parser avoids dependencies for RedrivePolicy
