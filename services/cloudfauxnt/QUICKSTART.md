# CloudFauxnt Quick Start Guide

Welcome to CloudFauxnt! This guide will get you up and running in minutes.

## What is CloudFauxnt?

CloudFauxnt is a CloudFront emulator that adds CloudFront-like features to your local development:
- ✅ Signed URL validation
- ✅ CORS handling
- ✅ Multi-origin routing
- ✅ CloudFront headers

## Quick Start (3 Steps)

### 1. Configure Environment

Set the required environment variables. You can do this in your shell, in a `.env` file, or in your `docker-compose.yml`:

```bash
export PORT=9310
export ORIGINS='[{"name":"s3","url":"http://essthree:9300","path_patterns":["/test-bucket/*"]}]'
```

The default configuration proxies requests matching `/test-bucket/*` directly to `http://essthree:9300`.

### 2. Run Both Services in Docker

CloudFauxnt and ess-three run as separate Docker containers connected via a shared network.

**Terminal 1: Start ess-three**
```bash
cd /path/to/ess-three
docker compose up -d
```

**Terminal 2: Start CloudFauxnt**
```bash
cd /path/to/CloudFauxnt
docker compose up -d
```

**Check they're both running:**
```bash
docker ps
# Should show both cloudfauxnt and ess-three containers
```

### 3. Test It

```bash
# Health check
curl http://localhost:9310/health
# Should return: {"status":"healthy","service":"cloudfauxnt"}

# Test proxy with direct path forwarding
curl http://localhost:9310/test-bucket/MyTestFile.txt
# This proxies to: http://essthree:9300/test-bucket/MyTestFile.txt
```

## Configuration Basics

Configure CloudFauxnt's behavior via environment variables:

```bash
# Server settings
export PORT=9310
export HOST=0.0.0.0

# Origins (JSON array)
export ORIGINS='[{"name":"s3","url":"http://essthree:9300","path_patterns":["/test-bucket/*"],"default_root_object":"index.html"}]'

# CORS
export CORS_ENABLED=true

# Signing (disabled by default)
export SIGNING_ENABLED=false
```

**Path Forwarding Example:**
```
Request:  http://localhost:9310/test-bucket/MyTestFile.txt
Matches:  /test-bucket/*
Proxied:  http://essthree:9300/test-bucket/MyTestFile.txt
```

## Running Separate Containers

CloudFauxnt, ess-three, and ess-queue-ess are separate Docker services that communicate via a shared Docker network (`shared-network`):

```bash
# Create the shared network (once)
docker network create shared-network

# Start ess-three (from ess-three directory)
cd /path/to/ess-three && docker compose up -d

# Start ess-queue-ess (from ess-queue-ess directory)
cd /path/to/ess-queue-ess && docker compose up -d

# Start CloudFauxnt (from Cloudfauxnt directory)
cd /path/to/CloudFauxnt && docker compose up -d

# View logs
docker logs cloudfauxnt -f
docker logs ess-three -f

# Stop services
docker compose down  # (from either directory)

# Restart
docker compose restart
```

**Hostname Resolution:**
- Inside Docker: `http://ess-three:9000`, `http://ess-queue-ess:9324`, `http://cloudfauxnt:9310` (service names)
- From host machine: `http://localhost:9000`, `http://localhost:9324`, `http://localhost:9310` (port mappings)

**Network Connection Troubleshooting:**
If containers don't connect to the shared network on first startup, manually connect them:
```bash
docker network connect shared-network ess-three
docker network connect shared-network ess-queue-ess
docker network connect shared-network cloudfauxnt
```
- CloudFauxnt ORIGINS env var uses: `http://essthree:9300` (runs in Docker)

## Running Locally (No Docker)

You can also run both services locally without Docker:

**Terminal 1: ess-three**
```bash
cd /path/to/ess-three
./ess-three
```

**Terminal 2: CloudFauxnt**
```bash
cd /path/to/CloudFauxnt
go build -o cloudfauxnt .
export PORT=9310
export ORIGINS='[{"name":"s3","url":"http://127.0.0.1:9300","path_patterns":["/test-bucket/*"]}]'
./cloudfauxnt
```

## Next Steps

### Enable CloudFront Signing

1. **Generate RSA keys:**
   ```bash
   cd /path/to/CloudFauxnt/keys
   openssl genrsa -out private.pem 2048
   openssl rsa -in private.pem -pubout -out public.pem
   cd ..
   ```

2. **Set signing environment variables:**
   ```bash
   export SIGNING_ENABLED=true
   export SIGNING_KEY_PAIR_ID=APKAJEXAMPLE123456
   export SIGNING_PUBLIC_KEY_PATH=/app/keys/public.pem  # Path inside Docker container
   ```

3. **Rebuild and restart CloudFauxnt:**
   ```bash
   docker compose down
   docker compose build --no-cache
   docker compose up -d
   ```

4. **Generate and test a signed URL:**
   See [keys/README.md](keys/README.md) for Python example

### Add Multiple Origins

```bash
export ORIGINS='[
  {"name":"s3","url":"http://essthree:9300","path_patterns":["/test-bucket/*","/other-bucket/*"]},
  {"name":"api","url":"https://api.example.com","path_patterns":["/api/*"]},
  {"name":"default","url":"http://localhost:3000","path_patterns":["/*"]}
]'
```

Requests to `/api/users` go to `api.example.com`, `/test-bucket/key` goes to S3, everything else goes to localhost:3000.

### Mixed Security Levels (Per-Origin Signatures)

When `SIGNING_ENABLED=true`, you can allow unsigned access to specific origins:

```bash
export SIGNING_ENABLED=true
export SIGNING_KEY_PAIR_ID=APKAJEXAMPLE123456
export SIGNING_PUBLIC_KEY_PATH=/app/keys/public.pem

export ORIGINS='[
  {"name":"public-files","url":"http://essthree:9300","path_patterns":["/public/*"],"require_signature":false},
  {"name":"premium-content","url":"http://essthree:9300","path_patterns":["/premium/*"],"require_signature":true},
  {"name":"temp-downloads","url":"http://essthree:9300","path_patterns":["/temp/*"]}
]'
```

Now `/public/file.txt` works unsigned, but `/premium/file.txt` requires a signature. `/temp/*` inherits the global `SIGNING_ENABLED` setting.

### Configure CORS

Enable CORS via the `CORS_ENABLED` environment variable:

```bash
export CORS_ENABLED=true
```

## Troubleshooting

### "Connection refused" when testing

- Check CloudFauxnt is running: `docker compose ps` or `ps aux | grep cloudfauxnt`
- Check port 9310 is not in use: `lsof -i :9310`

### "No origin found for path"

- Check your `path_patterns` in the `ORIGINS` env var
- Patterns match longest-first
- Use `/*` as a catch-all

### Signature validation fails

- Verify `key_pair_id` matches between config and signing code
- Check expiration time is in the future
- Ensure public key is valid: `openssl rsa -in keys/public.pem -pubin -text`
- Check system time is synchronized: `date` should show correct time

### CORS errors in browser

- Check `allowed_origins` includes your app's origin
- Use `["*"]` for development
- Verify `CORS_ENABLED=true` is set

## Example: Full Stack with ess-three

```yaml
# docker-compose.yml
version: '3.8'

services:
  essthree:
    image: essthree:latest
    ports:
      - "9300:9300"
    volumes:
      - ./data:/data
    networks:
      - app

  cloudfauxnt:
    build: .
    ports:
      - "9310:9310"
    environment:
      PORT: "9310"
      ORIGINS: '[{"name":"s3","url":"http://essthree:9300","path_patterns":["/*"]}]'
      CORS_ENABLED: "true"
    volumes:
      - ./keys:/app/keys:ro
    depends_on:
      - essthree
    networks:
      - app

networks:
  app:
```

Then your apps connect to `http://localhost:9310` instead of `http://localhost:9300`.

## More Resources

- [Full README](README.md) - Detailed documentation
- [keys/README.md](keys/README.md) - Signing key generation and usage
- [test/README.md](test/README.md) - Testing guide

## Getting Help

- Check logs: `docker compose logs cloudfauxnt` or `journalctl -u cloudfauxnt`
- Test health: `curl http://localhost:9310/health`
- Verify environment: ensure `ORIGINS` is set and contains valid JSON

Happy coding! 🎉
