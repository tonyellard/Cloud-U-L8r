# kay-vee: API Compatibility Matrix (MVP)

This matrix defines the first implementation target for `kay-vee`.

Concrete request/response examples are documented in [KAY_VEE_REQUEST_RESPONSE_SHAPES.md](KAY_VEE_REQUEST_RESPONSE_SHAPES.md).

## Scope Priority
Primary focus:
- create values
- modify values
- retrieve values

Secondary (included where low-friction):
- delete values
- list/describe metadata

Explicitly deferred:
- rotation orchestration
- deep IAM/policy behavior

---

## Parameter Store (SSM)

| Operation | MVP Status | Notes |
|---|---|---|
| `PutParameter` | Must Have | Supports create and overwrite (`Overwrite=true`) |
| `GetParameter` | Must Have | Supports plain name, label, version selectors |
| `GetParameters` | Must Have | Batch fetch by names |
| `GetParametersByPath` | Must Have | Supports recursive path retrieval |
| `DeleteParameter` | Should Have | Single delete |
| `DeleteParameters` | Should Have | Batch delete |
| `LabelParameterVersion` | Should Have | Needed for realistic version flows |
| `DescribeParameters` | Later | Metadata query may come after retrieval flows |
| `GetParameterHistory` | Later | Optional if version retrieval is already sufficient |

### SSM Data Semantics (MVP)
- `String`, `StringList`, `SecureString`
- Version increments on update
- Label mapping per parameter
- `SecureString` retrieved only when `WithDecryption=true` (emulated)

---

## Secrets Manager

| Operation | MVP Status | Notes |
|---|---|---|
| `CreateSecret` | Must Have | Initial create with `SecretString` or `SecretBinary` |
| `GetSecretValue` | Must Have | Retrieval by secret id + optional version selectors |
| `PutSecretValue` | Must Have | New version value writes |
| `UpdateSecret` | Must Have | Metadata/value update behavior |
| `DescribeSecret` | Should Have | Useful for tooling parity |
| `ListSecrets` | Should Have | Useful for UI + debugging |
| `DeleteSecret` | Should Have | Simplified soft-delete acceptable |
| `RestoreSecret` | Should Have | If soft delete implemented |
| `UpdateSecretVersionStage` | Later | Add if needed by client workflows |
| `RotateSecret` | Deferred | Out of scope for current goals |

### Secrets Data Semantics (MVP)
- `SecretString` and `SecretBinary`
- Version ids on writes
- Staging labels:
  - new version promoted to `AWSCURRENT`
  - prior current becomes `AWSPREVIOUS`

---

## Emulator-Only Admin Endpoints

| Endpoint | Purpose |
|---|---|
| `GET /health` | Liveness |
| `GET /admin/api/summary` | Dashboard counters |
| `GET /admin/api/export` | Export state for local reproducibility |
| `POST /admin/api/import` | Import pre-seeded state |
| `GET /admin/api/activity` | Auditable operation log |

---

## Definition of Done (MVP)
- Parameter Store create/modify/retrieve flows work with representative SDK calls
- Secrets Manager create/modify/retrieve flows work with representative SDK calls
- Version behavior is deterministic and test-covered
- `SecureString` and secret payload handling documented and test-covered
- Basic delete/list/describe flows implemented for local dev ergonomics
- Admin summary/export endpoints available for console integration
