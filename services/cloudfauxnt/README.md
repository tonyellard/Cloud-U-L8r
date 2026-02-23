# CloudFauxnt

A lightweight CloudFront emulator for local development, providing CloudFront-like features in front of S3 emulators or other backend services.

## Features

- **CloudFront Signed URLs** - Validate canned policy signed URLs with RSA-SHA1
- **CloudFront Signed Cookies** - Support for CloudFront-Policy, CloudFront-Signature, CloudFront-Key-Pair-Id
- **CORS Handling** - Full preflight and origin validation support
- **Multi-Origin Routing** - Route requests to different backends based on path patterns
- **CloudFront Headers** - Inject realistic CloudFront headers (X-Amz-Cf-Id, Via, X-Cache)
- **Docker Ready** - Multi-stage Debian builds with minimal image size
- **Simple Configuration** - Environment variable configuration with optional YAML fallback

## Quick Start

### 1. Configure Environment

Set the required environment variables (e.g., in your `docker-compose.yml` or shell):

```bash
export PORT=9310
export HOST=0.0.0.0
export ORIGINS='[{"name":"s3","url":"http://essthree:9300","path_patterns":["/test-bucket/*"],"require_signature":false,"default_root_object":"index.html"}]'
```

### 2. Create Shared Network

All three services (CloudFauxnt, ess-three, and ess-queue-ess) use a shared Docker bridge network for local development:

```bash
docker network create shared-network
```

### 3. Start Services

Start each service in its own terminal. They will automatically connect to the shared network:

**Terminal 1: Start ess-three**
```bash
cd /path/to/essthree
docker compose up -d
```

**Terminal 2: Start ess-queue-ess**
```bash
cd /path/to/ess-queue-ess
docker compose up -d
```

**Terminal 3: Start CloudFauxnt**
```bash
cd /path/to/CloudFauxnt
docker compose up -d

# Check health
curl http://localhost:9310/health
```

Services can now communicate using container names:

- **ess-three**: `http://essthree:9300`
- **ess-queue-ess**: `http://ess-queue-ess:9320`
- **cloudfauxnt**: `http://cloudfauxnt:9310`

### 4. Generate RSA Keys (if using signing)

```bash
cd keys
openssl genrsa -out private.pem 2048
openssl rsa -in private.pem -pubout -out public.pem
cd ..
```

### 4. Configure

Set environment variables to match your environment:

```bash
export PORT=9310
export HOST=0.0.0.0
export ORIGINS='[{"name":"s3","url":"http://essthree:9300","path_patterns":["/test-bucket/*"],"require_signature":false,"default_root_object":"index.html"}]'
export SIGNING_ENABLED=true
export SIGNING_KEY_PAIR_ID=APKAJEXAMPLE123456
export SIGNING_PUBLIC_KEY_PATH=/app/keys/public.pem
```

## Examples

### .NET Example Client

A complete .NET 10 application demonstrating CloudFauxnt usage with unsigned requests, signed URLs, and signed cookies:

```bash
cd dotnet-example
dotnet run
```

Outputs:
- Health check via unsigned request
- File retrieval via direct path forwarding
- Signed URL generation and validation
- Signed cookie generation and usage

See [dotnet-example/README.md](dotnet-example/README.md) for detailed documentation and code samples.

### Manual Testing with curl

**Health check:**
```bash
curl http://localhost:9310/health
# {"status":"healthy","service":"cloudfauxnt"}
```

**Unsigned request:**
```bash
curl http://localhost:9310/test-bucket/MyTestFile.txt
# Hello World
```

**Signed URL request:**
```bash
# Generate signature using keys and policy
curl "http://localhost:9310/test-bucket/file.txt?Expires=1234567890&Signature=...&Key-Pair-Id=APKAJEXAMPLE123456"
```

## Usage

### Path Routing

CloudFauxnt routes requests to backend origins based on path pattern matching. Paths are forwarded directly to the origin without any rewriting:

**Example flow:**
```
Client request:     http://localhost:9310/test-bucket/document.pdf
Matches pattern:    /test-bucket/*
Proxies to:         http://essthree:9300/test-bucket/document.pdf
```

### Without Signature Validation

If signing is disabled, CloudFauxnt acts as a simple reverse proxy:

```bash
# Direct request (proxied to origin)
curl http://localhost:9310/test-bucket/myfile.txt
```

### With Signed URLs

Enable signing via environment variables:

```bash
export SIGNING_ENABLED=true
export SIGNING_KEY_PAIR_ID=APKAJEXAMPLE123456
export SIGNING_PUBLIC_KEY_PATH=/app/keys/public.pem
```

Generate a signed URL using your private key:

```python
# Python example
import time
import base64
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import padding

def create_signed_url(url, key_pair_id, private_key_path, expires_in=3600):
    expires = int(time.time()) + expires_in
    policy = f"{url}?Expires={expires}"
    
    with open(private_key_path, 'rb') as f:
        private_key = serialization.load_pem_private_key(f.read(), password=None)
    
    signature = private_key.sign(policy.encode(), padding.PKCS1v15(), hashes.SHA1())
    encoded_sig = base64.b64encode(signature).decode()
    
    return f"{url}?Expires={expires}&Signature={encoded_sig}&Key-Pair-Id={key_pair_id}"

# Usage
signed_url = create_signed_url(
    "http://localhost:9310/bucket/myfile.txt",
    "APKAJEXAMPLE123456",
    "keys/private.pem"
)
print(signed_url)
```

Request the signed URL:

```bash
curl "http://localhost:9310/bucket/myfile.txt?Expires=1234567890&Signature=...&Key-Pair-Id=APKAJEXAMPLE123456"
```

### With CORS

CloudFauxnt handles CORS automatically:

```bash
# Preflight request
curl -X OPTIONS \
  -H "Origin: http://localhost:3000" \
  -H "Access-Control-Request-Method: GET" \
  http://localhost:9310/bucket/myfile.txt

# Actual request with Origin header
curl -H "Origin: http://localhost:3000" \
  http://localhost:9310/bucket/myfile.txt
```

## Configuration Reference

Configuration is done via environment variables. An optional YAML config file can be loaded as a fallback using the `CONFIG_PATH` env var.

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `9310` | Port to listen on |
| `HOST` | `0.0.0.0` | Host to bind to |
| `TIMEOUT_SECONDS` | `30` | Request timeout |
| `CORS_ENABLED` | `false` | Enable CORS handling |
| `SIGNING_ENABLED` | `false` | Enable CloudFront signature validation |
| `SIGNING_KEY_PAIR_ID` | | Key pair ID for signature validation |
| `SIGNING_PUBLIC_KEY_PATH` | | Path to RSA public key PEM file |
| `ORIGINS` | | JSON array of origin definitions (see below) |
| `CONFIG_PATH` | | Optional path to a YAML config file (fallback) |

### Origins

Origins are configured via the `ORIGINS` env var as a JSON array:

```bash
export ORIGINS='[{"name":"s3","url":"http://essthree:9300","path_patterns":["/test-bucket/*"],"require_signature":false,"default_root_object":"index.html"}]'
```

Each origin object supports these fields:

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Friendly name for the origin |
| `url` | Yes | Backend service URL |
| `path_patterns` | Yes | Array of path patterns to match |
| `default_root_object` | No | Override global default root object for this origin |
| `require_signature` | No | Override global signing requirement for this origin |

#### Per-Origin Configuration

Each origin can override server-level defaults:

- **default_root_object** (optional): If set, this origin will serve this object when "/" is requested. Useful when different origins have different directory structures.
- **require_signature** (optional): If set (true/false), overrides the global `SIGNING_ENABLED` setting for this origin only. Allows mixed security models where some paths require signatures while others don't.

**Pattern Matching:**
- Exact match: `/health` matches only `/health`
- Prefix wildcard: `/test-bucket/*` matches `/test-bucket/key`
- Catch-all: `/*` matches everything
- Longest pattern wins (first match if equal length)

### Per-Origin Signature Enforcement

Override the global signature requirement on a per-origin basis to allow mixed security levels:

```bash
export SIGNING_ENABLED=true
export SIGNING_KEY_PAIR_ID=APKAJEXAMPLE123456
export SIGNING_PUBLIC_KEY_PATH=/app/keys/public.pem

export ORIGINS='[
  {"name":"public-bucket","url":"http://essthree:9300","path_patterns":["/public/*"],"require_signature":false},
  {"name":"private-bucket","url":"http://essthree:9300","path_patterns":["/private/*"],"require_signature":true},
  {"name":"protected-bucket","url":"http://essthree:9300","path_patterns":["/protected/*"]}
]'
```

- `public-bucket`: Override global setting, allow unsigned access
- `private-bucket`: Override global setting, explicitly require signatures
- `protected-bucket`: Omit `require_signature` to inherit the global `SIGNING_ENABLED` setting

**Signature Requirement Logic:**
1. If `require_signature` is set on the origin, use that value
2. Otherwise, use the global `SIGNING_ENABLED` setting
3. When a signature is required but missing/invalid, CloudFauxnt returns 403 Forbidden

**Real-World Example:**
- Public downloads: `/public/*` → `require_signature: false` (allow unsigned)
- Temporary links: `/download/*` → Use global setting (inherited)
- Premium content: `/premium/*` → `require_signature: true` (always require)

### CORS

CORS is enabled via the `CORS_ENABLED` environment variable:

```bash
export CORS_ENABLED=true
```

### Signing

Signing is configured via environment variables:

```bash
export SIGNING_ENABLED=true
export SIGNING_KEY_PAIR_ID=APKAJEXAMPLE123456
export SIGNING_PUBLIC_KEY_PATH=/app/keys/public.pem
```

## Integration with ess-three

CloudFauxnt is designed to work with [ess-three](../essthree), a lightweight S3 emulator.

### Separate Docker Containers (Recommended)

Run both as separate Docker services on a shared network:

**ess-three docker-compose.yml:**
```yaml
version: '3.8'
services:
  essthree:
    build: .
    container_name: essthree
    ports:
      - "9300:9300"
    volumes:
      - ./data:/data
    networks:
      - shared-network

networks:
  shared-network:
    external: true
```

**CloudFauxnt docker-compose.yml:**
```yaml
version: '3.8'
services:
  cloudfauxnt:
    build: .
    container_name: cloudfauxnt
    ports:
      - "9310:9310"
    environment:
      PORT: "9310"
      ORIGINS: '[{"name":"s3","url":"http://essthree:9300","path_patterns":["/*"]}]'
    volumes:
      - ./keys:/app/keys:ro
    networks:
      - shared-network

networks:
  shared-network:
    external: true
```

**Start both services:**
```bash
# Terminal 1
cd /path/to/essthree && docker compose up -d

# Terminal 2
cd /path/to/CloudFauxnt && docker compose up -d

# Verify both are running
docker ps | grep -E "cloudfauxnt|essthree"
```

**CloudFauxnt configuration (via environment):**
```bash
export ORIGINS='[{"name":"s3","url":"http://essthree:9300","path_patterns":["/*"]}]'
```

## Architecture

CloudFauxnt runs as a separate Docker container that proxies requests to origin services (like ess-three).

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│  Docker Bridge Network (shared-network)                         │
│                                                                 │
│  ┌──────────────────────┐         ┌──────────────────────┐     │
│  │  CloudFauxnt (9310)  │         │  essthree (9300)     │     │
│  │                      │         │                      │     │
│  │ • Validate Signature │         │ • S3 Emulator        │     │
│  │ • Check CORS         │─────────│ • Stores objects     │     │
│  │ • Route paths        │         │   in ./data dir      │     │
│  │ • Proxy requests     │         │                      │     │
│  └──────────┬───────────┘         └──────────────────────┘     │
│             │                                                   │
│             │ http://essthree:9300                              │
│             │ (Docker service name)                             │
│             │                                                   │
└─────────────┼───────────────────────────────────────────────────┘
              │
              │ Port mapping
              │
              ▼
        localhost:9310  (host access)
        localhost:9300
```

**Request Flow:**
1. Client sends request to CloudFauxnt: `/test-bucket/key?Signature=...`
2. CloudFauxnt validates signature and CORS
3. CloudFauxnt matches the path to an origin via path patterns
4. CloudFauxnt proxies the request directly to: `http://essthree:9300/test-bucket/key`
5. ess-three returns object from local storage

**Key Points:**
- Both containers run on the same Docker bridge network (`shared-network`)
- CloudFauxnt accesses ess-three via service name `essthree`, not localhost
- Paths are forwarded directly to the origin (no rewriting)
- Each service has its own docker-compose.yml file in separate directories

## Testing

### Run Unit Tests

```bash
go test ./...
```

### Integration Testing

```bash
# Start both services
cd /home/tony/Documents/essthree && docker compose up -d
cd /home/tony/Documents/Cloudfauxnt && docker compose up -d

# Wait for startup
sleep 2

# Test health endpoint
curl http://localhost:9310/health

# Test unsigned request
curl -v http://localhost:9310/test-bucket/MyTestFile.txt

# Test CORS preflight
curl -X OPTIONS \
  -H "Origin: http://localhost:3000" \
  -H "Access-Control-Request-Method: GET" \
  -v http://localhost:9310/test-bucket/MyTestFile.txt

# View logs
docker logs cloudfauxnt -f
docker logs essthree -f
```

### Testing Token Expiration and Clock Skew

To test expiration validation and clock skew tolerance:

```bash
# Test with a token that expires in 25 seconds
# (should pass with default 30-second clock skew)
curl "http://localhost:9310/test-bucket/file.txt?Expires=$(($(date +%s) + 25))&Signature=...&Key-Pair-Id=..."

# Test with a token that's already expired
# (should fail)
curl "http://localhost:9310/test-bucket/file.txt?Expires=$(($(date +%s) - 60))&Signature=...&Key-Pair-Id=..."
```

### Testing Per-Origin Signature Enforcement

To test mixed security levels with different paths:

```bash
export SIGNING_ENABLED=true
export SIGNING_KEY_PAIR_ID=APKAJEXAMPLE123456
export SIGNING_PUBLIC_KEY_PATH=/app/keys/public.pem
export ORIGINS='[
  {"name":"public","url":"http://essthree:9300","path_patterns":["/public/*"],"require_signature":false},
  {"name":"private","url":"http://essthree:9300","path_patterns":["/private/*"],"require_signature":true}
]'
```

Test behavior:

```bash
# Public path - should work without signature
curl http://localhost:9310/public/file.txt
# ✅ 200 OK

# Private path without signature
curl http://localhost:9310/private/file.txt
# ❌ 403 Forbidden - AccessDenied

# Private path with valid signature
curl "http://localhost:9310/private/file.txt?Expires=...&Signature=...&Key-Pair-Id=..."
# ✅ 200 OK
```

## Development

### Project Structure

```
Cloudfauxnt/
├── main.go              # Entry point, server setup
├── config.go            # Configuration parsing & validation
├── signing.go           # CloudFront signature validation
├── cors.go              # CORS middleware
├── handlers.go          # HTTP handlers and proxying
├── Dockerfile           # Multi-stage Docker build
├── docker-compose.yml   # Container orchestration
├── go.mod              # Go dependencies
├── keys/
│   └── README.md       # Key generation instructions
└── test/
    └── integration_test.py  # Integration tests
```

### Building

```bash
# Local build
go build -o cloudfauxnt .

# Docker build
docker build -t cloudfauxnt:latest .

# Multi-platform build
docker buildx build --platform linux/amd64,linux/arm64 -t cloudfauxnt:latest .
```

## Troubleshooting

### Docker Network Connection Issues

**Containers can't reach each other:**
- Verify all containers are on the shared network: `docker ps --format "table {{.Names}}\t{{.Networks}}"`
- If a container is not on the shared network, manually connect it: `docker network connect shared-network <container-name>`
- Use service names (e.g., `http://essthree:9300` from within CloudFauxnt) not `localhost` or `127.0.0.1`
- Check all services are running: `docker ps` should show cloudfauxnt, ess-three, and ess-queue-ess containers

**Container not connecting to external network on docker compose up:**
- Docker Compose sometimes fails to connect containers to external networks on the first `up` call
- Workaround 1: Manually connect: `docker network connect shared-network <container-name>`
- Workaround 2: Restart the service: `docker compose down && docker compose up -d`
- Ensure your docker-compose.yml has `external: true` for the shared-network definition

**Error: "dial tcp [::1]:9300: connect: connection refused"**
- This means the client resolved "localhost" to IPv6, but the service only listens on IPv4
- Fix: Use Docker service names (e.g., `http://essthree:9300`) instead of `http://localhost:9300`

**Testing connectivity:**
```bash
# From CloudFauxnt container
docker exec cloudfauxnt curl -v http://essthree:9300/health

# From ess-three container
docker exec essthree curl -v http://cloudfauxnt:9310/health
```

### Path Routing Not Working

**Requests going to wrong origin:**
- Verify that `path_patterns` in your `ORIGINS` env var match the request paths
- After changing environment variables, restart CloudFauxnt: `docker compose restart cloudfauxnt`
- If code changes were made, rebuild without cache: `docker compose build --no-cache && docker compose up -d`
- Check logs to see which origin was matched: `docker logs cloudfauxnt | grep "path"`

### Signature Validation Fails

- Verify the key pair ID matches between your signing code and config
- Check that the public key is valid: `openssl rsa -in public.pem -pubin -text`
- Ensure expiration time is in the future (Unix timestamp)
- Verify signature is base64-encoded correctly

### CORS Issues

- Check `allowed_origins` includes the requesting origin
- For development, use `["*"]` to allow all origins
- Verify the browser is sending an `Origin` header

### Origin Connection Fails (Non-Docker)

- For local development: ensure origin runs on correct port (e.g., `:9300`)
- For Docker: use service hostname, not localhost (see "Docker Network Connection Issues" above)
- Check origin service logs for errors: `docker logs essthree -f`

### Windows/WSL Build Issues

If you see errors like "installsuffix" during Docker build:
- Ensure `.gitattributes` is present and enforcing LF line endings
- Run `git config core.autocrlf false` in the repository
- Clone the repository fresh after changing the setting

## Roadmap

- [ ] Custom CloudFront policies (beyond canned policy)
- [ ] IP address restrictions in policies
- [ ] Response caching with TTL
- [ ] Metrics and Prometheus integration
- [ ] Request/response logging
- [ ] TLS/HTTPS support
- [ ] Admin API for runtime inspection

## Limitations

CloudFauxnt is a development tool with some intentional limitations:

- **No authentication/authorization** - All requests are accepted (intended for local development)
- **No request caching** - Every request is proxied in real-time
- **No CloudFront behaviors** - Advanced CloudFront features like behaviors, distributions not emulated
- **No S3 Select/Query** - Cannot query object contents
- **Simplified request signing** - Only validates CloudFront-compatible signatures, not AWS Signature V4
- **No request logging** - Minimal logging for debugging
- **Single configuration** - Configuration is static, cannot be changed at runtime

## Support

**Getting Help:** [TBD - Issue tracker and discussion board to be added]

**Reporting Issues:** [TBD - Contribution guidelines to be added]

## License

Licensed under the Apache License, Version 2.0. See `LICENSE`.

## Related Projects

- [ess-three](../essthree) - Lightweight S3 emulator for local development

## Contributing

Contributions welcome! Please:
1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Submit a pull request

## License

Licensed under the Apache License, Version 2.0. See the root [LICENSE](../../LICENSE) file for details.

## Trademark Notice

This project is not affiliated with, endorsed by, or sponsored by Amazon Web Services (AWS). Amazon S3, Amazon CloudFront, Amazon Secrets Manager, Amazon Parameter Store are all trademarks of amazon.com, Inc., or it's affiliates.

## Support

For issues and questions:
- GitHub Issues: [Report a bug](https://github.com/yourusername/cloudfauxnt/issues)
- Documentation: See this README and `keys/README.md`
