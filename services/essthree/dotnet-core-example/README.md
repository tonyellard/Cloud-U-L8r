# ess-three .NET Core Example

This example demonstrates how to use the ess-three S3 emulator with a .NET Core application.

## Prerequisites

- .NET 8.0 SDK or later
- ess-three emulator running (see main README)

## Setup

### 1. Restore dependencies

```bash
cd EssThreeExample
dotnet restore
```

### 2. Start the emulator

In another terminal:

```bash
cd ../..
docker-compose up -d
```

Verify it's running:

```bash
curl http://localhost:9300/health
```

### 3. Run the example

```bash
dotnet run
```

## What the example does

The example tests the following S3 operations:

### PutObject
- Uploads a file with content and content-type
- Returns ETag and HTTP status

### HeadObject
- Retrieves object metadata without downloading content
- Shows content-type, size, and last modified time

### GetObject
- Downloads a complete object
- Verifies content matches what was uploaded

### ListObjects
- Lists all objects in a bucket
- Shows object keys and sizes

### PutObject with Metadata
- Uploads a file with custom x-amz-meta-* headers
- Retrieves metadata with HeadObject to verify

### DeleteObject
- Deletes a single object

### DeleteObjects (Batch Delete)
- Creates multiple test files
- Deletes them all in a single batch operation

## Expected Output

```
=== ess-three .NET Core Example ===
Endpoint: http://localhost:9300
Bucket: test-bucket
====================================

Testing PutObject...
✓ PutObject successful
  ETag: "18-1704067200"
  HTTP Status: OK

Testing HeadObject...
✓ HeadObject successful
  Content-Type: text/plain
  Content-Length: 31
  Last Modified: [timestamp]
  ETag: "18-1704067200"

[... more test output ...]

=== All tests passed! ===
```

## Troubleshooting

### "Unable to connect to endpoint"

Make sure the emulator is running:

```bash
docker-compose up -d
curl http://localhost:9300/health
```

If port 9300 is in use, edit `Program.cs` to point to the correct port and update `docker-compose.yml`.

### "Access Denied" or authentication errors

The emulator ignores credentials for local development. Make sure you're using:
- Access Key: `test`
- Secret Key: `test`

These are set in the `Program.cs` and can be any value.

### Object not found errors

Make sure the bucket name matches what you're using. The example uses `test-bucket`.

## SDK Documentation

For more information about the AWS SDK for .NET, see:
- https://docs.aws.amazon.com/sdk-for-net/
- https://github.com/aws/aws-sdk-net
