# kay-vee: Storage Model (MVP)

This document defines the internal data model for `kay-vee`, focused on deterministic create/modify/retrieve behavior for both Parameter Store and Secrets Manager emulation.

## Design Goals
- Deterministic version behavior
- Fast lookups by canonical identifiers
- Clear label/stage resolution (`name:label`, `name:version`, `AWSCURRENT`)
- Simple persistence strategy (in-memory first, file snapshot optional)

---

## Shared Concepts

### Time
- Store all timestamps in UTC.
- Persist timestamps as RFC3339 strings in snapshots.

### IDs
- Parameter version: monotonically increasing `int64` per parameter name.
- Secret version id: UUID string per secret write.

### Thread Safety
- Use `sync.RWMutex` around top-level stores.
- Keep write operations atomic per logical resource.

### Admin Activity Log (MVP)
- Keep an in-memory append-only activity list for API calls and admin endpoints.
- Store entries with: timestamp, HTTP method, path, target, status code, and optional AWS error type.
- Maintain bounded history (fixed max size) by evicting oldest records when full.
- Expose retrieval in reverse-chronological order with `maxResults` + `nextToken` pagination.

---

## Parameter Store Model

### Canonical Key
- Parameter name (e.g. `/app/dev/db/password`) is the canonical key.

### Structures (conceptual)

```go
type ParameterRecord struct {
    Name             string
    Type             string // String | StringList | SecureString
    CurrentVersion   int64
    Versions         map[int64]ParameterVersion
    LabelsToVersions map[string]int64
    LastModifiedAt   time.Time
}

type ParameterVersion struct {
    Version         int64
    ValueCiphertext string // for SecureString; plain for others in MVP
    ValuePlaintext  string // optional cache for non-secure paths
    CreatedAt       time.Time
    Tier            string
    DataType        string
}
```

### Indexing Rules
- `parametersByName map[string]*ParameterRecord`
- No secondary index required for MVP lookups.
- Path queries (`GetParametersByPath`) are prefix scans over `Name`.

### GetParametersByPath Semantics (Current)
- Path must be absolute (begin with `/`); malformed hierarchy paths are rejected.
- `Recursive=false` returns only direct children under the path.
- `Recursive=true` returns all descendants.
- Returned list is sorted by parameter name before pagination.
- `MaxResults` is bounded to `<= 10`; `NextToken` is offset-based.
- Supported `ParameterFilters` keys: `Type`, `Label` (`Equals` option).

### Version/Label Rules
- `PutParameter` increments `CurrentVersion` on successful write.
- `Overwrite=false` fails if record exists.
- Label assignment maps label -> exact version.
- `GetParameter` name parsing supports:
  - `/path/name`
  - `/path/name:label`
  - `/path/name:version`

### SecureString Semantics (MVP)
- Use emulator-local reversible encryption abstraction.
- `WithDecryption=false` returns encrypted/placeholder value.
- `WithDecryption=true` returns plaintext.
- This is compatibility behavior, not KMS parity.

---

## Secrets Manager Model

### Canonical Key
- Secret name (or ARN alias) resolves to one canonical `SecretRecord`.

### Structures (conceptual)

```go
type SecretRecord struct {
    Name             string
    ARN              string
    Description      string
    KmsKeyID         string
    DeletedAt        *time.Time
    Versions         map[string]SecretVersion        // versionId -> data
    StageToVersion   map[string]string               // AWSCURRENT -> versionId
    VersionToStages  map[string]map[string]struct{}  // versionId -> set(stages)
    CreatedAt        time.Time
    LastChangedAt    time.Time
}

type SecretVersion struct {
    VersionID    string
    SecretString *string
    SecretBinary []byte
    CreatedAt    time.Time
}
```

### Indexing Rules
- `secretsByName map[string]*SecretRecord`
- `secretNameByARN map[string]string`
- ARN-based lookup resolves to name first, then accesses `secretsByName`.

### Version/Stage Rules
- `CreateSecret` creates initial version and assigns `AWSCURRENT`.
- `PutSecretValue` creates new version id.
- On successful new version write:
  - previous `AWSCURRENT` gets `AWSPREVIOUS`
  - new version gets `AWSCURRENT`
- Stage resolution priority:
  1. explicit `VersionId`
  2. explicit `VersionStage`
  3. fallback `AWSCURRENT`

### Delete/Restore Rules (MVP)
- `DeleteSecret` sets `DeletedAt` (soft-delete).
- Soft-deleted secrets are hidden from standard list/get unless restore path used.
- `RestoreSecret` clears `DeletedAt`.

---

## Persistence Snapshot (Optional MVP)

### Snapshot Envelope

```json
{
  "format_version": 1,
  "saved_at": "2026-02-13T23:00:00Z",
  "parameters": {},
  "secrets": {}
}
```

### Write Strategy
- Full snapshot write on interval and graceful shutdown.
- Write to temp file + atomic rename.

### Read Strategy
- Load snapshot at startup if present.
- Validate `format_version`; reject unknown versions.

---

## Error Mapping Guidance

### Parameter Store
- Missing name -> `ParameterNotFound`
- Duplicate create without overwrite -> `ParameterAlreadyExists`
- Invalid selector -> `ValidationException`
- Invalid/malformed path and unsupported `GetParametersByPath` filter keys/options -> `ValidationException`

### Secrets Manager
- Missing secret -> `ResourceNotFoundException`
- Deleted secret access -> `InvalidRequestException`
- Invalid stage/version combination -> `InvalidParameterException`

---

## Testing Requirements from Model
- Parameter version increment and retrieval by label/version
- `GetParametersByPath` recursion and deterministic ordering
- SecureString decryption flag behavior
- Secret version creation and stage transitions (`AWSCURRENT`/`AWSPREVIOUS`)
- ARN + name lookup equivalence
- Soft delete + restore behavior
