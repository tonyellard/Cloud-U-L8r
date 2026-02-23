# CLAUDE.md — Cloud-U-L8r

## Project Overview

Cloud-U-L8r is a monorepo of **six interconnected AWS service emulators** for local development, built in Go and orchestrated via Docker Compose. Each service emulates a specific AWS API (S3, CloudFront, SQS, SNS, SSM Parameter Store + Secrets Manager) plus a consolidated admin console.

## Repository Structure

```
Cloud-U-L8r/
├── configs/                         # Named Terraform config sets
│   └── default/                     # Default dev resource definitions (*.tf)
├── docs/                            # Architecture docs (kay-vee planning)
├── pkg/                             # Shared Go packages
│   ├── awserrors/                   # AWS-compatible error formatting (XML, JSON, CloudFront)
│   └── health/                      # Standardized health check handler
├── services/
│   ├── essthree/                    # S3 emulator (port 9300)
│   ├── cloudfauxnt/                 # CloudFront emulator (port 9310)
│   ├── ess-queue-ess/               # SQS emulator (port 9320)
│   ├── ess-enn-ess/                 # SNS emulator (port 9330, admin 9331)
│   ├── kay-vee/                     # SSM Parameter Store + Secrets Manager (port 9350)
│   └── admin-console/               # Consolidated admin dashboard (port 9999)
├── tests/integration/               # Cross-service integration tests
├── docker-compose.yml               # Full stack orchestration
├── Makefile                         # Primary build/run interface
├── go.work                          # Go workspace (all 6 services + shared packages)
├── start-stack.sh                   # Stack startup script
├── cleanup-stack.sh                 # Comprehensive container cleanup
├── verify-stack.sh                  # Health-check all services
└── test-cleanup.sh                  # Test cleanup procedure
```

## Services

| Service | Port | Emulates | Router | Entry Point |
|---------|------|----------|--------|-------------|
| **essthree** | 9300 | S3 | chi | `cmd/ess-three/main.go` |
| **cloudfauxnt** | 9310 | CloudFront | chi | `cmd/cloudfauxnt/main.go` |
| **ess-queue-ess** | 9320 | SQS | chi | `cmd/ess-queue-ess/main.go` |
| **ess-enn-ess** | 9330 | SNS | stdlib | `cmd/ess-enn-ess/main.go` |
| **kay-vee** | 9350 | SSM + Secrets Manager | stdlib | `cmd/kay-vee/main.go` |
| **admin-console** | 9999 | Admin dashboard | chi | `cmd/admin-console/main.go` |

### Service Internal Layouts

All services follow the same `cmd/` + `internal/` layout pattern:

```
service/
├── cmd/<service-name>/main.go    # Entry point
├── internal/
│   ├── server/                   # HTTP handlers and routing
│   ├── storage/                  # Data storage layer (where applicable)
│   ├── model/                    # Data types (where applicable)
│   └── <domain>/                 # Domain packages (ess-enn-ess has topic/, subscription/, delivery/, etc.)
├── Dockerfile
└── go.mod
```

## Build and Run Commands

**Always use `make` targets — never run `docker compose` directly.**

| Command | Description |
|---------|-------------|
| `make build` | Build all Docker images (cached) |
| `make rebuild` | Clean + rebuild all images (no cache) |
| `make up` | Build and start all services |
| `make down` | Stop all services and clean up stray containers |
| `make logs` | Stream logs from all containers |
| `make test` | Run unit tests (Go) + integration tests (bash) |
| `make clean` | Full reset: remove containers, volumes, networks, images |
| `make status` | Show Docker container status |
| `make stop-service SERVICE=<name>` | Stop a single service |
| `make start-service SERVICE=<name>` | Start a single service |
| `make restart-service SERVICE=<name>` | Restart a single service |
| `make stack` | Build + start + apply default Terraform config |
| `make run-config CONFIG=<name>` | Apply a named Terraform config |
| `make tf-init CONFIG=<name>` | Initialize Terraform for a config |
| `make tf-plan CONFIG=<name>` | Plan changes for a config |
| `make tf-destroy CONFIG=<name>` | Destroy resources for a config |

## Testing

### Unit Tests

Run Go tests per-service:
```bash
cd services/<service-name> && go test ./...
```

Or all at once:
```bash
make test
```

Unit tests use `httptest.NewRequest` / `httptest.NewRecorder` for HTTP handler testing. Services with Go unit tests: **essthree**, **cloudfauxnt**, **ess-queue-ess**, **ess-enn-ess**, **kay-vee**, **admin-console**.

### Integration Tests

- `tests/integration/test_cross_service.sh` — Cross-service integration (health checks, S3 access, CloudFront proxy, SQS operations)
- Service-specific bash test scripts live inside each service directory (e.g., `ess-queue-ess/test_quick.sh`, `ess-enn-ess/test_publish.sh`, `kay-vee/test/aws_cli_smoke.sh`)

Integration tests require the stack to be running (`make up` first).

### Test Patterns

**Go unit tests:**
```go
req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
req.Header.Set("X-Amz-Target", "AmazonSSM.PutParameter")
rr := httptest.NewRecorder()
router.ServeHTTP(rr, req)
// Assert on rr.Code, rr.Body
```

**Bash integration tests:**
```bash
curl -s http://localhost:9300/health
curl -X PUT http://localhost:9300/test-bucket/key -d "content"
```

## Go Workspace

The root `go.work` file declares a workspace over all 6 service modules. Standard Go commands work from the repo root:
```bash
go build ./services/essthree/...
go test ./services/kay-vee/...
```

Go version: **1.23** for all services. Workspace requires Go **1.24.7+**.

### Dependencies

Services are intentionally minimal in external dependencies:

| Service | External Dependencies |
|---------|----------------------|
| essthree | `go-chi/chi` v5, `pkg/awserrors`, `pkg/health` |
| cloudfauxnt | `go-chi/chi` v5, `google/uuid`, `gopkg.in/yaml.v3`, `pkg/health` |
| ess-queue-ess | `go-chi/chi` v5, `google/uuid`, `pkg/awserrors`, `pkg/health` |
| ess-enn-ess | `gopkg.in/yaml.v3`, `pkg/health` |
| kay-vee | `pkg/awserrors`, `pkg/health` |
| admin-console | `go-chi/chi` v5, `pkg/awserrors`, `pkg/health` |

## Docker

### Multi-Stage Build Pattern

All Dockerfiles follow a two-stage pattern:
1. **Builder stage** — `golang:1.23-alpine` or `golang:1.23-bookworm` base, copies `go.mod`/`go.sum`, downloads deps, builds binary
2. **Runtime stage** — `alpine:latest` or `debian:bookworm-slim`, copies binary + LICENSE/NOTICE

### Docker Compose Dependency Order

```
essthree  (standalone)
  └── cloudfauxnt (depends_on: essthree)

ess-queue-ess (standalone)
  └── ess-enn-ess (depends_on: ess-queue-ess)

kay-vee (standalone)

admin-console (depends_on: ess-queue-ess, ess-enn-ess, kay-vee)
```

Inter-service communication uses container names on `shared-network` (e.g., `http://essthree:9300`).

## Configuration

Services are configured via **environment variables** set in `docker-compose.yml`. AWS resources (buckets, queues, topics, subscriptions) are provisioned via **Terraform** configs in `configs/[config-name]/`.

- **essthree** — Env vars: `DATA_DIR`
- **cloudfauxnt** — Env vars: `PORT`, `HOST`, `CORS_ENABLED`, `SIGNING_ENABLED`, `SIGNING_KEY_PAIR_ID`, `SIGNING_PUBLIC_KEY_PATH`, `ORIGINS` (JSON)
- **ess-queue-ess** — Env vars: `PORT`
- **ess-enn-ess** — Env vars: `API_PORT`, `ADMIN_PORT`, `HOST`, `REGION`, `ACCOUNT_ID`, `SQS_ENDPOINT`, `SQS_ENABLED`, `AUTO_CONFIRM_SUBSCRIPTIONS`
- **kay-vee** — Env vars: `PORT` (in-memory only)
- **admin-console** — Env vars: `PORT` (uses hardcoded service endpoints)

Named Terraform configs live in `configs/[config-name]/` (e.g., `configs/default/`). Apply with `make run-config CONFIG=default`.

## Code Conventions

### Commit Messages

Follow conventional-commit-style prefixes:
```
feat(<scope>): description          # New feature
fix(<scope>): description           # Bug fix
docs: description                   # Documentation only
test(<scope>): description          # Adding/updating tests
ui: description                     # UI/frontend changes
legal: description                  # License/legal changes
```

Scope is typically the service name: `kay-vee`, `admin-console`, `essthree`, `cloudfauxnt`, `ess-enn-ess`, `ess-queue-ess`. Multi-service changes use comma-separated scopes (e.g., `feat(kay-vee,admin-console):`).

### License Headers

All source files must include the SPDX license identifier:
```go
// SPDX-License-Identifier: Apache-2.0
```

YAML/Makefile files use `#` comment style:
```yaml
# SPDX-License-Identifier: Apache-2.0
```

### Handler Patterns

- Handlers are methods on a `Server` struct: `func (s *Server) handleXxx(w http.ResponseWriter, r *http.Request)`
- AWS-style APIs dispatch on the `X-Amz-Target` header (kay-vee, ess-queue-ess JSON protocol)
- S3 uses XML request/response format; all other services use JSON
- SQS supports dual protocol: form-encoded (Query protocol) and JSON (`X-Amz-Target` header)
- All services expose a `/health` endpoint

### Logging

- Newer services (ess-enn-ess, kay-vee, admin-console): `log/slog` structured logging
- Older services (essthree, cloudfauxnt, ess-queue-ess): `log` or `fmt` printf-style

### Error Responses

Services return AWS-compatible error formats:
- **essthree**: XML `<Error>` responses matching S3 API
- **All others**: JSON `{"__type": "ErrorType", "message": "..."}` matching AWS JSON protocol

### Storage

- **essthree**: Filesystem-based (`/data` directory with bucket/key structure)
- **ess-queue-ess, ess-enn-ess, kay-vee**: In-memory stores (no persistence across restarts; use `make run-config` to re-provision resources)

### Admin APIs

Every service provides admin endpoints:
- `/health` — Health check
- `/admin` — Admin UI (HTML)
- `/admin/api/*` — Admin REST API (JSON) for inspection, export, import

## Port Scheme

All services use the 93xx range with 10-port increments:

| Port | Service |
|------|---------|
| 9300 | essthree (S3) |
| 9310 | cloudfauxnt (CloudFront) |
| 9320 | ess-queue-ess (SQS) |
| 9330 | ess-enn-ess (SNS API) |
| 9331 | ess-enn-ess (SNS Admin UI) |
| 9350 | kay-vee (Parameter Store + Secrets Manager) |
| 9999 | admin-console |

## Key Files for Common Tasks

| Task | Files to Look At |
|------|-----------------|
| Add a new S3 operation | `services/essthree/internal/server/handlers.go`, `server.go` |
| Modify CloudFront proxy | `services/cloudfauxnt/internal/server/handlers.go`, `config.go` |
| Add SQS action | `services/ess-queue-ess/internal/server/handlers.go`, `queue.go` |
| Add SNS operation | `services/ess-enn-ess/internal/server/handlers.go` |
| Add SSM/Secrets Manager operation | `services/kay-vee/internal/server/router.go`, `internal/storage/store.go` |
| Modify admin dashboard | `services/admin-console/internal/server/server.go`, `web/` directory |
| Add shared error handling | `pkg/awserrors/errors.go` |
| Add shared health behavior | `pkg/health/health.go` |
| Change service config | `docker-compose.yml` (env vars), `configs/default/*.tf` (resources) |
| Update Docker build | `services/<service>/Dockerfile`, `docker-compose.yml` |
| Add integration test | `tests/integration/test_cross_service.sh` |

## Branching and PR Conventions

- Feature branches: `feature/your-feature-name`
- Bugfix branches: `bugfix/<service-or-description>`
- PRs target `master`
- Run `make test` before submitting PRs
