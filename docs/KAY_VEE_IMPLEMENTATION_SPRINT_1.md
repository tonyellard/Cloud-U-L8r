# kay-vee: Implementation Sprint 1

Sprint 1 goal: produce a runnable `kay-vee` service with the first end-to-end create/modify/retrieve workflows for both Parameter Store and Secrets Manager.

## Sprint Objective
By end of Sprint 1, developers can:
- create/update/get parameters
- create/update/get secrets
- verify deterministic version behavior
- run basic integration checks locally

## In-Scope Operations (Sprint 1)

### Parameter Store
- `PutParameter`
- `GetParameter`
- `GetParameters`

### Secrets Manager
- `CreateSecret`
- `GetSecretValue`
- `PutSecretValue`

## Out of Scope (Sprint 1)
- Path listing (`GetParametersByPath`)
- Delete/restore flows
- Describe/list metadata endpoints
- Admin-console integration

## Proposed File Scaffold

- `services/kay-vee/`
  - `go.mod`
  - `Dockerfile`
  - `README.md`
  - `cmd/kay-vee/main.go`
  - `internal/server/router.go`
  - `internal/server/handlers.go`
  - `internal/storage/store.go`
  - `internal/storage/parameter_store.go`
  - `internal/storage/secret_store.go`
  - `internal/crypto/secure.go`
  - `internal/model/types.go`
  - `internal/config/config.go`
  - `internal/server/handlers_test.go`
  - `internal/storage/store_test.go`

## Implementation Sequence

1. **Scaffold service runtime**
   - HTTP server
   - health endpoint
   - config parsing
2. **Storage core**
   - in-memory store structs
   - mutex-protected read/write paths
3. **Parameter handlers**
   - implement put/get/get-batch
   - add version increment semantics
4. **Secret handlers**
   - implement create/get/put-value
   - stage transition (`AWSCURRENT`/`AWSPREVIOUS`)
5. **Error mapping**
   - map internal errors to AWS-like envelope
6. **Tests + docker wiring**
   - unit tests for version/stage behavior
   - local container run verification

## Acceptance Criteria
- Service starts and responds on `:9350`
- All Sprint 1 operations return valid JSON shapes from [KAY_VEE_REQUEST_RESPONSE_SHAPES.md](KAY_VEE_REQUEST_RESPONSE_SHAPES.md)
- Parameter versions increment deterministically on overwrite
- Secret writes create new version ids and transition stages correctly
- Unit tests cover success + key error paths

## Risks and Mitigations
- **Risk**: AWS protocol shape drift
  - **Mitigation**: Keep explicit fixtures for request/response examples
- **Risk**: Concurrency bugs in version increments
  - **Mitigation**: lock per write path and add race-oriented tests
- **Risk**: Ambiguity in secure value behavior
  - **Mitigation**: document emulator semantics clearly in README and tests

## Deliverables
- Initial runnable service in `services/kay-vee`
- Passing unit tests for Sprint 1 operations
- README quickstart for local usage
