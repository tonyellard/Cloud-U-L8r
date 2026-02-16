# kay-vee: Test Plan (MVP)

This plan defines required verification for the MVP scope focused on create/modify/retrieve workflows.

## Test Layers

## 1) Unit Tests (Required)

### Parameter Store
- `PutParameter` creates version `1`
- `PutParameter` with `Overwrite=true` increments version
- `PutParameter` with `Overwrite=false` on existing returns expected error
- `GetParameter` by name returns latest version
- `GetParameter` by `name:version` resolves correct historical value
- `GetParameter` by `name:label` resolves correct labeled version
- `GetParameters` returns found values and invalid list separation

### Secrets Manager
- `CreateSecret` creates initial version and `AWSCURRENT`
- `PutSecretValue` creates new version id
- Stage transition correctness: old current -> `AWSPREVIOUS`, new -> `AWSCURRENT`
- `GetSecretValue` default resolves `AWSCURRENT`
- `GetSecretValue` by explicit version id resolves deterministic version

### Shared/Infra
- Error mapping to AWS-like envelope types
- Thread-safe concurrent writes maintain consistent version increments

## 2) Handler/Protocol Tests (Required)
- Target header routing (`AmazonSSM.*`, `secretsmanager.*`)
- JSON request validation and missing-field handling
- Response shape fidelity to [KAY_VEE_REQUEST_RESPONSE_SHAPES.md](KAY_VEE_REQUEST_RESPONSE_SHAPES.md)

## 3) Integration Tests (Sprint 2+ minimum)
- Boot service in test mode
- Execute a representative Parameter Store flow:
  - put -> update -> get latest -> get by version
- Execute a representative Secrets flow:
  - create -> put value -> get current -> get previous

## Test Data Conventions
- Use stable test names:
  - parameters under `/test/...`
  - secrets under `test/...`
- Freeze timestamps where needed for deterministic assertions.
- UUIDs may be asserted by format + existence, not hard-coded value.

## CI Expectations
- Unit tests run on every PR
- Race detector for storage package
- Integration suite gated or nightly initially

## MVP Exit Criteria
- All required unit + handler tests green
- No flaky tests under repeated execution
- Version/stage behavior proven by tests for both services
