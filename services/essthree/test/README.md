# Test Scripts

This directory contains integration and end-to-end tests for ess-three.

## Python Integration Tests

Tests the S3 API using boto3 (AWS SDK for Python).

### Prerequisites

```bash
pip install boto3
```

### Running

```bash
# Start the emulator
docker-compose up -d

# Run tests
python test/integration_test.py

# Stop the emulator
docker-compose down
```

## Go Unit Tests

```bash
# Run all unit tests
go test ./...

# Run with coverage
go test -cover ./...

# Run with verbose output
go test -v ./...
```

## Manual Testing with AWS CLI

```bash
# Configure (use dummy credentials)
aws configure set aws_access_key_id test
aws configure set aws_secret_access_key test
aws configure set region us-east-1

# Upload
echo "Hello World" > test.txt
aws s3 cp test.txt s3://mybucket/test.txt --endpoint-url=http://localhost:9300

# List
aws s3 ls s3://mybucket/ --endpoint-url=http://localhost:9300

# Download
aws s3 cp s3://mybucket/test.txt downloaded.txt --endpoint-url=http://localhost:9300

# Delete
aws s3 rm s3://mybucket/test.txt --endpoint-url=http://localhost:9300
```

## Nested Sync Regression Scenario

This scenario reproduces and verifies the `aws s3 sync` nested-folder path behavior.

```bash
# Start ess-three first
docker-compose up -d

# Run nested sync smoke test
chmod +x test/nested_sync_smoke.sh
test/nested_sync_smoke.sh
```

The script creates nested local folders, runs `aws s3 sync`, and validates nested keys with `head-object`.

## Expected Results

All tests should pass with output similar to:

```
============================================================
ess-three Integration Tests
============================================================
Endpoint: http://localhost:9300
Bucket: test-bucket
============================================================
Testing PutObject... ✓ PASSED
Testing GetObject... ✓ PASSED
Testing HeadObject... ✓ PASSED
Testing ListObjectsV2... ✓ PASSED
Testing metadata preservation... ✓ PASSED
Testing DeleteObject... ✓ PASSED
============================================================
Results: 6/6 tests passed
============================================================
✓ All tests passed!
```

## Trademark Notice

This project is not affiliated with, endorsed by, or sponsored by Amazon Web Services (AWS). Amazon S3, Amazon CloudFront, Amazon Secrets Manager, Amazon Parameter Store are all trademarks of amazon.com, Inc., or it's affiliates.
