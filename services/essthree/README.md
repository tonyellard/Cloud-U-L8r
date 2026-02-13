# ess-three - Local S3 Emulator

A lightweight, API-compatible S3 emulator for local development. Run it in a container and connect your applications using the AWS SDK with a custom endpoint.

## Features

- **S3 API Compatible**: Works with AWS SDKs and CLI
- **Persistent Storage**: File-based storage with Docker volumes
- **Lightweight**: Minimal resource footprint
- **Easy Setup**: Single container deployment

## Supported Operations

### Core Operations
- `PutObject` - Upload objects
- `GetObject` - Download objects with optional Range header support
- `HeadObject` - Get object metadata
- `ListObjectsV1` - List bucket contents with marker-based pagination
- `ListObjectsV2` - List bucket contents with continuation tokens
- `DeleteObject` - Remove single objects
- `DeleteObjects` - Batch delete multiple objects

### Advanced Features
- **Multipart Uploads** - Upload large files in parts
  - `CreateMultipartUpload` - Initiate multipart upload
  - `UploadPart` - Upload individual parts
  - `CompleteMultipartUpload` - Finalize upload
  - `AbortMultipartUpload` - Cancel upload
- **Range Requests** - Download partial object content (HTTP 206 Partial Content)
- **Pagination** - Both V1 (marker) and V2 (continuation tokens) formats

## Quick Start

### Using Docker Compose (Recommended)

```bash
docker-compose up -d
```

The service will be available at `http://localhost:9300`

**Note for Windows/WSL users**: If the build fails with an "installsuffix" error, see the [Troubleshooting](#troubleshooting) section below.

### Shared Networking with Other Services

Ess-Three, Ess-Queue-Ess, and Cloudfauxnt share a Docker bridge network for local development. This allows services to communicate with each other using container names.

First, create the shared network (once):

```bash
docker network create shared-network
```

Then start services from their respective directories. Each will automatically connect to the shared network. From inside a container, you can reach:

- **essthree**: `http://essthree:9300`
- **ess-queue-ess**: `http://ess-queue-ess:9320`
- **cloudfauxnt**: `http://cloudfauxnt:9310`

### Using Docker

```bash
# Build
docker build -t ess-three .

# Run
docker run -d -p 9300:9300 -v $(pwd)/data:/data ess-three
```

### Local Development

```bash
# Install dependencies
go mod download

# Run
go run cmd/ess-three/main.go --port=9300 --data-dir=./data
```

## Usage Examples

### AWS CLI

```bash
# Configure AWS CLI (credentials can be dummy values for local testing)
aws configure set aws_access_key_id test
aws configure set aws_secret_access_key test
aws configure set region us-east-1

# Upload a file
aws s3 cp file.txt s3://mybucket/file.txt --endpoint-url=http://localhost:9300

# Download a file
aws s3 cp s3://mybucket/file.txt downloaded.txt --endpoint-url=http://localhost:9300

# List objects
aws s3 ls s3://mybucket/ --endpoint-url=http://localhost:9300

# Delete a file
aws s3 rm s3://mybucket/file.txt --endpoint-url=http://localhost:9300
```

### Python (boto3)

```python
import boto3

# Create S3 client with custom endpoint
s3 = boto3.client(
    's3',
    endpoint_url='http://localhost:9300',
    aws_access_key_id='test',
    aws_secret_access_key='test',
    region_name='us-east-1'
)

# Upload a file
s3.put_object(Bucket='mybucket', Key='test.txt', Body=b'Hello World')

# Download a file
response = s3.get_object(Bucket='mybucket', Key='test.txt')
content = response['Body'].read()

# List objects
response = s3.list_objects_v2(Bucket='mybucket')
for obj in response.get('Contents', []):
    print(obj['Key'])
```

### Go (AWS SDK v2)

```go
package main

import (
    "context"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "strings"
)

func main() {
    cfg, _ := config.LoadDefaultConfig(context.TODO(),
        config.WithRegion("us-east-1"),
    )
    
    client := s3.NewFromConfig(cfg, func(o *s3.Options) {
        o.BaseEndpoint = aws.String("http://localhost:9300")
        o.UsePathStyle = true
    })
    
    // Upload
    client.PutObject(context.TODO(), &s3.PutObjectInput{
        Bucket: aws.String("mybucket"),
        Key:    aws.String("test.txt"),
        Body:   strings.NewReader("Hello World"),
    })
}
```

### Node.js (AWS SDK v3)

```javascript
const { S3Client, PutObjectCommand, GetObjectCommand } = require("@aws-sdk/client-s3");

const client = new S3Client({
    endpoint: "http://localhost:9300",
    region: "us-east-1",
    credentials: {
        accessKeyId: "test",
        secretAccessKey: "test"
    },
    forcePathStyle: true
});

// Upload
await client.send(new PutObjectCommand({
    Bucket: "mybucket",
    Key: "test.txt",
    Body: "Hello World"
}));

// Download
const response = await client.send(new GetObjectCommand({
    Bucket: "mybucket",
    Key: "test.txt"
}));
```

## Configuration

### Environment Variables

- `PORT` - Server port (default: 9300)
- `DATA_DIR` - Data storage directory (default: /data)

### Command Line Flags

```bash
./ess-three --port=9300 --data-dir=/data
```

## Data Storage

Objects are stored in the filesystem with the following structure:

```
/data/
  └── mybucket/
      ├── objects/
      │   └── file.txt
      └── metadata/
          └── file.txt.json
```

Metadata includes:
- Object key
- Size
- ETag
- Content-Type
- Last modified timestamp
- Custom metadata (x-amz-meta-* headers)

## Testing

See the `test/` directory for integration tests.

```bash
# Run tests
go test ./...

# Run integration tests (requires running server)
cd test
python integration_test.py
```

## Troubleshooting

### Docker Build Fails on Windows/WSL: "installsuffix" Error

If you see an error like:
```
ERROR: Failed to build: failed to solve: process "/bin/sh -c CGO_ENABLED=0 GOOS=linux go build..." did not complete successfully
```

This is usually caused by Git converting line endings on Windows. Fix it:

```bash
# Option 1: Clone with correct settings
git config --global core.autocrlf false
git clone https://github.com/tonyellard/Ess-Three.git
cd Ess-Three
docker compose build --no-cache

# Option 2: Fix existing clone
git checkout --quiet --force --recursive
docker compose build --no-cache
```

The `.gitattributes` file ensures correct line endings across platforms.

### Other Issues

- **Port 9300 in use**: Edit `docker-compose.yml` and change the port mapping
- **Permission denied on `/data`**: Run `chmod 755 ./data` (Linux/Mac)
- **Objects disappear after restart**: Ensure Docker volumes are configured in `docker-compose.yml`

## Limitations

This is a development tool and has some limitations:

- **No authentication/authorization** - All requests accepted (intended for local development)
- **No versioning** - Each object has only one version
- **No bucket policies or ACLs** - No fine-grained access control
- **No S3 Select/Query** - Cannot query object contents
- **No tagging** - Object tags not supported
- **No request signing validation** - AWS Signature V4 not validated
- **No S3 events** - No event notifications
- **Simplified storage** - Single filesystem backend, not replicated

## Support

**Getting Help:** [TBD - Issue tracker and discussion board to be added]

**Reporting Issues:** [TBD - Contribution guidelines to be added]

## License

Licensed under the Apache License, Version 2.0. See the root [LICENSE](../../LICENSE) file for details.

## Trademark notice

Not affiliated with, endorsed by, or sponsored by Amazon Web Services (AWS).
Amazon S3 and Amazon CloudFront are trademarks of Amazon.com, Inc. or its affiliates.
