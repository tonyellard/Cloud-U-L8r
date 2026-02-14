# kay-vee - Local SSM + Secrets Manager Emulator

`kay-vee` is a local emulator for AWS Systems Manager Parameter Store and AWS Secrets Manager.

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
