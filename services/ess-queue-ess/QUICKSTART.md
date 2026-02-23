# QUICKSTART

Get Ess-Queue-Ess running in under 5 minutes.

## Prerequisites

- Docker and Docker Compose (recommended)
- OR Go 1.23+ (for local development)

## Quick Start with Docker

1. **Start the stack**:
   ```bash
   cd /path/to/cloud-u-l8r
   make up
   ```

2. **Provision queues via Terraform**:
   ```bash
   make run-config CONFIG=default
   ```

3. **Verify it's running**:
   ```bash
   curl http://localhost:9320/health
   # Should return: {"status":"healthy"}
   ```

4. **Open the Admin UI**:
   Open your browser to `http://localhost:9320/admin` to see the web interface.

5. **Create your first queue** (using AWS CLI):
   ```bash
   aws sqs create-queue --queue-name my-first-queue --endpoint-url http://localhost:9320
   ```

6. **Send a message**:
   ```bash
   aws sqs send-message \
     --queue-url http://localhost:9320/my-first-queue \
     --message-body "Hello, Ess-Queue-Ess!" \
     --endpoint-url http://localhost:9320
   ```

7. **Receive the message**:
   ```bash
   aws sqs receive-message \
     --queue-url http://localhost:9320/my-first-queue \
     --endpoint-url http://localhost:9320
   ```

8. **View in Admin UI**: Refresh the admin page to see your new queue and message!

## Using the Admin UI for Queue Management

The admin UI at `http://localhost:9320/admin` provides a visual way to manage queues:

### Create a Queue
1. Click "**+ Create Queue**" button
2. Fill in queue settings:
   - Queue Name (required)
   - Visibility Timeout (default: 30 seconds)
   - Message Retention Period (default: 4 days)
   - Maximum Message Size (default: 256 KB)
3. Click "**Create Queue**"

### Send Test Messages
1. Find the queue in the list
2. Click "**📤 Send**" button
3. Enter your message body and optional delay
4. Click "**Send Message**"

### Delete a Queue
1. Find the queue in the list
2. Click "**🗑 Delete**" button
3. Confirm deletion

For consistent, reproducible queue setups, use Terraform provisioning (`make run-config CONFIG=default`) from the repository root.

## Quick Start with Go

1. **Build and run**:
   ```bash
   cd services/ess-queue-ess
   go build -o ess-queue-ess ./cmd/ess-queue-ess
   PORT=9320 ./ess-queue-ess
   ```

2. **Open Admin UI**: Visit `http://localhost:9320/admin` in your browser

3. **In another terminal, test the service**:
   ```bash
   # Create queue
   curl -X POST http://localhost:9320/ \
     -d "Action=CreateQueue&QueueName=test-queue"

   # Send message
   curl -X POST http://localhost:9320/ \
     -d "Action=SendMessage&QueueUrl=http://localhost:9320/test-queue&MessageBody=Hello"

   # Receive message
   curl -X POST http://localhost:9320/ \
     -d "Action=ReceiveMessage&QueueUrl=http://localhost:9320/test-queue&MaxNumberOfMessages=10"
   ```

## Provisioning Queues

Queues are provisioned via Terraform rather than YAML config files:

```bash
# From the repository root
make run-config CONFIG=default
```

You can also create queues at runtime using the SQS API or the Admin UI.

## Using with Your Application

### Python (boto3)

```python
import boto3

sqs = boto3.client('sqs',
    endpoint_url='http://localhost:9320',
    region_name='us-east-1',
    aws_access_key_id='test',
    aws_secret_access_key='test'
)

# Create queue
response = sqs.create_queue(QueueName='app-queue')
queue_url = response['QueueUrl']

# Send message
sqs.send_message(QueueUrl=queue_url, MessageBody='{"task": "process"}')

# Receive and process
messages = sqs.receive_message(QueueUrl=queue_url, MaxNumberOfMessages=10)
for msg in messages.get('Messages', []):
    print(f"Processing: {msg['Body']}")
    sqs.delete_message(QueueUrl=queue_url, ReceiptHandle=msg['ReceiptHandle'])
```

### JavaScript/Node.js (AWS SDK v3)

```javascript
import { SQSClient, CreateQueueCommand, SendMessageCommand, ReceiveMessageCommand } from "@aws-sdk/client-sqs";

const client = new SQSClient({
  endpoint: "http://localhost:9320",
  region: "us-east-1",
  credentials: { accessKeyId: "test", secretAccessKey: "test" }
});

// Create queue
const { QueueUrl } = await client.send(new CreateQueueCommand({ QueueName: "app-queue" }));

// Send message
await client.send(new SendMessageCommand({ QueueUrl, MessageBody: "Hello!" }));

// Receive messages
const { Messages } = await client.send(new ReceiveMessageCommand({ QueueUrl, MaxNumberOfMessages: 10 }));
```

### .NET (AWS SDK)

```csharp
using Amazon.SQS;
using Amazon.SQS.Model;

var config = new AmazonSQSConfig
{
    ServiceURL = "http://localhost:9320"
};
var client = new AmazonSQSClient(config);

// Create queue
var createResponse = await client.CreateQueueAsync("app-queue");
var queueUrl = createResponse.QueueUrl;

// Send message
await client.SendMessageAsync(queueUrl, "Hello from .NET!");

// Receive messages
var receiveResponse = await client.ReceiveMessageAsync(new ReceiveMessageRequest
{
    QueueUrl = queueUrl,
    MaxNumberOfMessages = 10
});
```

## Next Steps

- See [README.md](README.md) for full documentation
- Check supported operations and limitations
- View example .NET client in `dotnet-example/`

## Stopping the Service

```bash
# Docker
docker compose down

# Go (Ctrl+C in the terminal running the service)
```

That's it! You're ready to develop and test with Ess-Queue-Ess.
