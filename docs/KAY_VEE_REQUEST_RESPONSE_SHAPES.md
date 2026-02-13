# kay-vee: Request/Response Shapes (MVP)

This document provides concrete JSON shapes for the MVP operations marked **Must Have** in [KAY_VEE_API_COMPAT_MATRIX.md](KAY_VEE_API_COMPAT_MATRIX.md).

For internal persistence/indexing/version rules behind these shapes, see [KAY_VEE_STORAGE_MODEL.md](KAY_VEE_STORAGE_MODEL.md).

## Protocol Approach (MVP)

`kay-vee` will support AWS-style JSON APIs behind one service endpoint, with protocol routing by target header.

- Endpoint (proposed): `http://localhost:9350`
- Header pattern (illustrative):
  - Parameter Store: `X-Amz-Target: AmazonSSM.<Action>`
  - Secrets Manager: `X-Amz-Target: secretsmanager.<Action>`
- Content type: `application/x-amz-json-1.1`

MVP may accept equivalent emulator-friendly JSON POST routes internally if needed, but these shapes define compatibility targets.

---

## Parameter Store (SSM)

### `PutParameter`

**Request**
```json
{
  "Name": "/app/dev/db/host",
  "Type": "String",
  "Value": "localhost",
  "Overwrite": true,
  "Tier": "Standard"
}
```

**Response**
```json
{
  "Version": 3,
  "Tier": "Standard"
}
```

### `GetParameter`

**Request**
```json
{
  "Name": "/app/dev/db/password",
  "WithDecryption": true
}
```

**Response**
```json
{
  "Parameter": {
    "Name": "/app/dev/db/password",
    "Type": "SecureString",
    "Value": "super-secret",
    "Version": 5,
    "ARN": "arn:aws:ssm:us-east-1:000000000000:parameter/app/dev/db/password",
    "LastModifiedDate": "2026-02-13T22:00:00Z"
  }
}
```

### `GetParameters`

**Request**
```json
{
  "Names": [
    "/app/dev/db/host",
    "/app/dev/db/password"
  ],
  "WithDecryption": true
}
```

**Response**
```json
{
  "Parameters": [
    {
      "Name": "/app/dev/db/host",
      "Type": "String",
      "Value": "localhost",
      "Version": 3
    },
    {
      "Name": "/app/dev/db/password",
      "Type": "SecureString",
      "Value": "super-secret",
      "Version": 5
    }
  ],
  "InvalidParameters": []
}
```

### `GetParametersByPath`

**Request**
```json
{
  "Path": "/app/dev",
  "Recursive": true,
  "WithDecryption": true,
  "MaxResults": 10,
  "NextToken": null
}
```

**Response**
```json
{
  "Parameters": [
    {
      "Name": "/app/dev/db/host",
      "Type": "String",
      "Value": "localhost",
      "Version": 3
    },
    {
      "Name": "/app/dev/db/password",
      "Type": "SecureString",
      "Value": "super-secret",
      "Version": 5
    }
  ],
  "NextToken": null
}
```

---

## Secrets Manager

### `CreateSecret`

**Request**
```json
{
  "Name": "app/dev/db/credentials",
  "Description": "Database credentials for dev",
  "SecretString": "{\"username\":\"dev\",\"password\":\"super-secret\"}"
}
```

**Response**
```json
{
  "ARN": "arn:aws:secretsmanager:us-east-1:000000000000:secret:app/dev/db/credentials-AbCdEf",
  "Name": "app/dev/db/credentials",
  "VersionId": "f2b0d7d5-ff8c-44f6-a4f3-0df61338a9f2"
}
```

### `GetSecretValue`

**Request**
```json
{
  "SecretId": "app/dev/db/credentials",
  "VersionStage": "AWSCURRENT"
}
```

**Response**
```json
{
  "ARN": "arn:aws:secretsmanager:us-east-1:000000000000:secret:app/dev/db/credentials-AbCdEf",
  "Name": "app/dev/db/credentials",
  "VersionId": "f2b0d7d5-ff8c-44f6-a4f3-0df61338a9f2",
  "SecretString": "{\"username\":\"dev\",\"password\":\"super-secret\"}",
  "VersionStages": ["AWSCURRENT"],
  "CreatedDate": "2026-02-13T22:05:00Z"
}
```

### `PutSecretValue`

**Request**
```json
{
  "SecretId": "app/dev/db/credentials",
  "SecretString": "{\"username\":\"dev\",\"password\":\"new-secret\"}",
  "VersionStages": ["AWSCURRENT"]
}
```

**Response**
```json
{
  "ARN": "arn:aws:secretsmanager:us-east-1:000000000000:secret:app/dev/db/credentials-AbCdEf",
  "Name": "app/dev/db/credentials",
  "VersionId": "fef813af-ec07-48df-b039-c7f0814ec3b9",
  "VersionStages": ["AWSCURRENT"]
}
```

### `UpdateSecret`

**Request**
```json
{
  "SecretId": "app/dev/db/credentials",
  "Description": "Database credentials for dev environment",
  "SecretString": "{\"username\":\"dev\",\"password\":\"rotated-like-update\"}"
}
```

**Response**
```json
{
  "ARN": "arn:aws:secretsmanager:us-east-1:000000000000:secret:app/dev/db/credentials-AbCdEf",
  "Name": "app/dev/db/credentials",
  "VersionId": "e81e5ca6-5f8e-43c4-9a3f-c35eb7bbac7c"
}
```

---

## Error Shape (MVP)

For MVP compatibility, return AWS-like error envelopes where practical. For internal consistency, preserve a canonical error schema in logs/admin surfaces.

**Example error payload**
```json
{
  "__type": "ParameterNotFound",
  "message": "Parameter /app/dev/missing was not found"
}
```

---

## Notes for Implementation

- Parameter version increments must be deterministic on successful writes.
- Secret writes (`PutSecretValue` / value-changing `UpdateSecret`) should create new versions.
- Stage transitions:
  - newly written secret version becomes `AWSCURRENT`
  - prior `AWSCURRENT` becomes `AWSPREVIOUS`
- `SecureString` decryption behavior is emulated locally and should be documented as non-KMS parity.
