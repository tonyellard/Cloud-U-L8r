# Getting Started with ess-three

This guide will help you get started with ess-three, a local S3 emulator for development with AWS SDK compatibility.

## Features

- ✅ Core S3 operations (PUT, GET, DELETE, LIST)
- ✅ Multipart uploads (large files)
- ✅ Range requests (partial content)
- ✅ Batch delete operations
- ✅ ListObjects V1 & V2 with pagination
- ✅ Custom metadata and object attributes
- ✅ Docker containerization with persistent volumes

## Prerequisites

- Docker and Docker Compose (recommended)
- OR Go 1.23+ for local development

## Quick Start (Docker - Recommended)

### 1. Start the Emulator

```bash
docker-compose up -d
```

This will:
- Build the Docker image
- Start the container on port 9300
- Mount `./data` for persistent storage

### 2. Verify It's Running

```bash
curl http://localhost:9300/health
```

You should see: `OK`

### 3. Test with AWS CLI

First, configure AWS CLI with dummy credentials:

```bash
aws configure set aws_access_key_id test
aws configure set aws_secret_access_key test
aws configure set region us-east-1
```

Create a test file and upload it:

```bash
echo "Hello from ess-three!" > test.txt
aws s3 cp test.txt s3://mybucket/test.txt --endpoint-url=http://localhost:9300
```

List objects:

```bash
aws s3 ls s3://mybucket/ --endpoint-url=http://localhost:9300
```

Download the file:

```bash
aws s3 cp s3://mybucket/test.txt downloaded.txt --endpoint-url=http://localhost:9300
cat downloaded.txt
```

## Alternative: Local Development

### 1. Build and Run

```bash
make build
./ess-three --port=9300 --data-dir=./data
```

Or run directly:

```bash
make run
```

### 2. Run Tests

```bash
make test
```

## Using with Your Application

### Python (boto3)

```python
import boto3

s3 = boto3.client(
    's3',
    endpoint_url='http://localhost:9300',
    aws_access_key_id='test',
    aws_secret_access_key='test',
    region_name='us-east-1'
)

# Upload
with open('myfile.txt', 'rb') as f:
    s3.put_object(Bucket='mybucket', Key='myfile.txt', Body=f)

# Download
response = s3.get_object(Bucket='mybucket', Key='myfile.txt')
content = response['Body'].read()

# List
response = s3.list_objects_v2(Bucket='mybucket')
for obj in response.get('Contents', []):
    print(f"{obj['Key']} - {obj['Size']} bytes")
```

### Node.js (AWS SDK v3)

```javascript
const { S3Client, PutObjectCommand, GetObjectCommand, ListObjectsV2Command } = require("@aws-sdk/client-s3");
const fs = require('fs');

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
const fileContent = fs.readFileSync('myfile.txt');
await client.send(new PutObjectCommand({
    Bucket: "mybucket",
    Key: "myfile.txt",
    Body: fileContent
}));

// List
const list = await client.send(new ListObjectsV2Command({
    Bucket: "mybucket"
}));
console.log(list.Contents);
```

### Go (AWS SDK v2)

```go
package main

import (
    "context"
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "os"
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
    file, _ := os.Open("myfile.txt")
    defer file.Close()
    
    client.PutObject(context.TODO(), &s3.PutObjectInput{
        Bucket: aws.String("mybucket"),
        Key:    aws.String("myfile.txt"),
        Body:   file,
    })
}
```

## Advanced Features

### Multipart Uploads

Upload large files in parts:

```python
import boto3

s3 = boto3.client(
    's3',
    endpoint_url='http://localhost:9300',
    aws_access_key_id='test',
    aws_secret_access_key='test',
    region_name='us-east-1'
)

# Initiate upload
response = s3.create_multipart_upload(Bucket='mybucket', Key='large-file.bin')
upload_id = response['UploadId']

# Upload parts
with open('large-file.bin', 'rb') as f:
    parts = []
    for i in range(1, 4):
        part_data = f.read(5 * 1024 * 1024)  # 5MB chunks
        response = s3.upload_part(
            Bucket='mybucket',
            Key='large-file.bin',
            PartNumber=i,
            UploadId=upload_id,
            Body=part_data
        )
        parts.append({'ETag': response['ETag'], 'PartNumber': i})

# Complete upload
s3.complete_multipart_upload(
    Bucket='mybucket',
    Key='large-file.bin',
    UploadId=upload_id,
    MultipartUpload={'Parts': parts}
)
```

### Range Requests

Download partial object content:

```python
# Get first 100 bytes
response = s3.get_object(
    Bucket='mybucket',
    Key='myfile.txt',
    Range='bytes=0-99'
)
partial_content = response['Body'].read()
```

### Batch Delete

Delete multiple objects in one request:

```python
response = s3.delete_objects(
    Bucket='mybucket',
    Delete={
        'Objects': [
            {'Key': 'file1.txt'},
            {'Key': 'file2.txt'},
            {'Key': 'file3.txt'}
        ]
    }
)
```

### Pagination

List objects with pagination:

```python
# ListObjects V2 with continuation tokens
paginator = s3.get_paginator('list_objects_v2')
pages = paginator.paginate(Bucket='mybucket')

for page in pages:
    for obj in page.get('Contents', []):
        print(obj['Key'])
```

## Integration Tests

Run the Python integration tests:

```bash
# Install boto3
pip install boto3

# Make sure the emulator is running
docker-compose up -d

# Run tests
python test/integration_test.py
```

## Troubleshooting

### Build Fails on Windows/WSL with "installsuffix" Error

This is usually caused by Git line ending conversion. Fix it by:

```bash
# Clone fresh with proper settings
git config --global core.autocrlf false
git clone https://github.com/tonyellard/Ess-Three.git
cd Ess-Three
docker compose build --no-cache
```

Or if you already cloned it:

```bash
# Reset line endings
git checkout --quiet --force --recursive
docker compose build --no-cache
```

### Port Already in Use

If port 9300 is already in use, edit `docker-compose.yml`:

```yaml
ports:
  - "9001:9300"  # Change 9001 to any available port
```

Then use `http://localhost:9001` as your endpoint.

### Permission Denied on Data Directory

Make sure the `./data` directory is writable:

```bash
chmod 755 ./data
```

### Objects Disappear After Restart

Make sure you're using a volume mount in `docker-compose.yml`:

```yaml
volumes:
  - ./data:/data
```

## Next Steps

- See [README.md](README.md) for full documentation
- Check [test/README.md](test/README.md) for testing strategies
- Review supported S3 operations and limitations

## Stopping the Emulator

```bash
docker-compose down
```

To also remove the stored data:

```bash
docker-compose down -v
rm -rf ./data
```
