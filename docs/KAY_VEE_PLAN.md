# kay-vee: Scope and Architecture

## Goal
Build a single local emulator service (`kay-vee`) that combines:
- AWS Systems Manager Parameter Store behavior
- AWS Secrets Manager behavior

The service is intentionally unified for local development simplicity while exposing separate API-compatible surfaces.

The immediate planning and implementation priority is:
- creation/modification of parameters and secrets
- retrieval of current and versioned values

This means we intentionally prioritize fast, reliable local workflows over full AWS parity.

## Naming
- Service name: `kay-vee`
- Container/service key (planned): `kay-vee`
- Primary local endpoint (proposed): `http://localhost:9350`

## v1 Scope

### Parameter Store (SSM-style)
- `PutParameter` (create + overwrite/update)
- `GetParameter` (latest + with decryption flag)
- `GetParameters` (batch retrieval)
- `GetParametersByPath` (hierarchical retrieval)
- `DeleteParameter` / `DeleteParameters`
- Parameter versions and labels (`Name:label`, `Name:version` lookup)
- `SecureString` support with emulator-managed local encryption semantics

### Secrets Manager
- `CreateSecret`
- `GetSecretValue`
- `PutSecretValue` (modify secret values)
- `UpdateSecret` (metadata updates)
- `DeleteSecret` / `RestoreSecret` (soft delete behavior simplified)
- `DescribeSecret` / `ListSecrets`
- Secret versions with staging labels (`AWSCURRENT`, `AWSPREVIOUS`)
- `SecretString` and `SecretBinary` payload support

### Shared Capabilities
- In-memory first, optional file-backed persistence (configurable)
- Health endpoint and summary endpoint for admin-console cards
- Activity/audit logging for create/update/get/delete operations
- Export/import of current state for reproducible local environments

## Non-Goals (v1)
- IAM policy enforcement
- KMS integration parity
- Resource policies and cross-account simulation
- Secret rotation scheduling/Lambda orchestration
- Full advanced policy condition support
- Multi-region replication behaviors

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

Preference for implementation: AWS JSON protocol compatibility where feasible, with pragmatic fallback on clearly documented emulator-only admin endpoints.

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
2. Parameter Store MVP (put/get/get-by-path/delete + versions)
3. Secrets Manager MVP (create/get/put/update/delete + versions)
4. Unified persistence + export/import + audit log
5. Admin summary endpoints + dashboard integration
6. Tests (unit + integration for create/modify/retrieve paths)

## Open Decisions
- Single port vs split virtual endpoints behind one server
- Default persistence mode (memory vs file)
- Exact compatibility strategy (strict AWS wire format vs pragmatic compatibility)
- How to represent `SecureString`/secret encryption semantics in local mode

## Companion Docs
- [KAY_VEE_API_COMPAT_MATRIX.md](KAY_VEE_API_COMPAT_MATRIX.md)
- [KAY_VEE_REQUEST_RESPONSE_SHAPES.md](KAY_VEE_REQUEST_RESPONSE_SHAPES.md)
