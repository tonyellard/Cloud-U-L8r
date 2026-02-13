# kay-vee: Scope and Architecture

## Goal
Build a single local emulator service (`kay-vee`) that combines:
- AWS Systems Manager Parameter Store behavior
- AWS Secrets Manager behavior

The service is intentionally unified for local development simplicity while exposing separate API-compatible surfaces.

## Naming
- Service name: `kay-vee`
- Container/service key (planned): `kay-vee`
- Primary local endpoint (proposed): `http://localhost:9350`

## v1 Scope

### Parameter Store (SSM-style)
- Put/Get/Delete parameter
- Get parameters by path (hierarchical names)
- Labels and versions
- Basic filtering by path and recursion
- `SecureString` support via internal encryption abstraction (emulated)

### Secrets Manager
- Create/Get/Update/Delete secret
- Secret versions and staging labels (`AWSCURRENT`, `AWSPREVIOUS`)
- Binary + string secret payloads
- Describe/list secrets

### Shared Capabilities
- In-memory first, optional file-backed persistence (configurable)
- Activity/audit log stream for admin-console integration
- Config export/import for reproducible local state
- Health endpoint and summary endpoint for dashboard cards

## Non-Goals (v1)
- IAM policy enforcement
- KMS integration parity
- Resource policies and cross-account simulation
- Rotation Lambdas and scheduler parity

## Data Model Direction
Use one internal store with two logical namespaces:
- `parameters/*`
- `secrets/*`

This allows:
- Shared persistence mechanics
- Unified activity logging
- Separate API handlers and validation per surface

## API Surface Direction
- `/ssm/*` (or AWS-compatible target-based handler)
- `/secretsmanager/*` (or AWS-compatible target-based handler)
- `/admin/api/*` for emulator-only operator workflows

## Proposed Repo Layout
- `services/kay-vee/`
  - `cmd/kay-vee/main.go`
  - `internal/server/`
  - `internal/ssm/`
  - `internal/secrets/`
  - `internal/storage/`
  - `internal/activity/`
  - `README.md`

## Delivery Phases
1. Service scaffold + health + config loading
2. Parameter Store core operations
3. Secrets Manager core operations
4. Versioning/labels parity improvements
5. Admin APIs + export + dashboard integration
6. Tests (unit + integration)

## Open Decisions
- Single port vs split virtual endpoints behind one server
- Default persistence mode (memory vs file)
- Exact compatibility strategy (strict AWS wire format vs pragmatic compatibility)
- How to represent `SecureString`/secret encryption semantics in local mode
