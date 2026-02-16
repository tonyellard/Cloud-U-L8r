# kay-vee - Local SSM + Secrets Manager Emulator

`kay-vee` is a local emulator for AWS Systems Manager Parameter Store and AWS Secrets Manager.

## Current Status

`kay-vee` is active in the stack and integrated into `admin-console` for summary, activity, parameter workflows, and secret workflows.
Historical planning documents remain under `docs/` for reference, but `services/kay-vee/README.md` is the source-of-truth for currently supported behavior.

## Features

- AWS-style JSON RPC over `X-Amz-Target`
- In-memory storage for fast local development
- Parameter versioning with basic selector support (`name`, `name:version`, `name:label`)
- Secret version stages (`AWSCURRENT`, `AWSPREVIOUS`)
- Health endpoint for container orchestration checks

## Supported Operations

### Parameter Store (SSM)

- `PutParameter`
- `LabelParameterVersion`
- `GetParameter`
- `GetParameters`
- `GetParametersByPath`
- `DescribeParameters`
- `GetParameterHistory`
- `DeleteParameter`
- `DeleteParameters`

### Secrets Manager

- `CreateSecret`
- `GetSecretValue`
- `PutSecretValue`
- `UpdateSecret`
- `DescribeSecret`
- `ListSecrets`
- `DeleteSecret`
- `RestoreSecret`
- `UpdateSecretVersionStage`

Pagination support (`MaxResults`, `NextToken`) is available on list/describe/history-style operations.
Basic filtering support is available for:
- `DescribeParameters` via `ParameterFilters` (`Name`/`Type` with `Equals`/`Contains`/`BeginsWith`)
- `ListSecrets` via `Filters` (`name` contains matching)

`GetParametersByPath` compatibility notes:
- Supports `Path`, `Recursive`, `WithDecryption`, `MaxResults`, and `NextToken`.
- `ParameterFilters` supports `Type` and `Label` keys with `Equals` option.
- Path must be absolute (start with `/`) and `MaxResults` is capped at 10.
- Results are deterministic and sorted by parameter name before pagination.

Admin endpoints:
- `GET /admin/api/summary`
- `GET /admin/api/resources` (lists parameters and secrets for admin UI refresh flows)
- `GET /admin/api/activity` (supports `maxResults` and `nextToken` query params)
- `GET /admin/api/export`
- `POST /admin/api/import`

## Quick Start

### Local

```bash
go run ./cmd/kay-vee
```

Service default endpoint: `http://localhost:9350`

Health check:

```bash
curl http://localhost:9350/health
```

### Docker

```bash
docker build -t kay-vee .
docker run --rm -p 9350:9350 kay-vee
```

## Example Request

```bash
curl -s http://localhost:9350/ \
  -H 'Content-Type: application/x-amz-json-1.1' \
  -H 'X-Amz-Target: AmazonSSM.PutParameter' \
  -d '{"Name":"/app/dev/url","Type":"String","Value":"http://localhost","Overwrite":true}'
```

## Notes

- This emulator intentionally prioritizes local dev compatibility over strict AWS parity.
- Rotation workflows are currently out of scope.
- Some AWS edge-case validation and less-common filter keys/options are intentionally not implemented yet.

## Smoke Tests (Port 9350)

Two lightweight integration scripts are available under `test/`:

- `test/aws_cli_smoke.sh` (AWS CLI)
- `test/boto3_smoke.py` (Python + boto3)
- `test/dotnet-smoke` (.NET console + AWS SDK)

They each create, update, and retrieve one parameter and one secret against the running endpoint.

Run from `services/kay-vee`:

```bash
chmod +x test/aws_cli_smoke.sh
./test/aws_cli_smoke.sh
python3 test/boto3_smoke.py
dotnet run --project test/dotnet-smoke
```

Prerequisites for smoke tests:
- AWS CLI (for `aws_cli_smoke.sh`)
- Python 3 + `boto3` (for `boto3_smoke.py`)
- .NET SDK (for `test/dotnet-smoke`)

## License

Licensed under the Apache License, Version 2.0. See the root [LICENSE](../../LICENSE) file for details.

## Trademark Notice

This project is not affiliated with, endorsed by, or sponsored by Amazon Web Services (AWS). Amazon S3, Amazon CloudFront, Amazon Secrets Manager, Amazon Parameter Store are all trademarks of amazon.com, Inc., or it's affiliates.
