# Docker Compose Conflict Fix - Full Stack Startup

## Problem
The subdirectories `essthree/`, `ess-queue-ess/`, and `cloudfauxnt/` each had their own `docker-compose.yml` files that conflicted with the main `docker-compose.yml` at the root level, causing:
- Duplicate service definitions
- cloudfauxnt not starting (dependency on essthree failing)
- Multiple copies of ess-queue-ess being attempted

## Solution
All subdirectory docker-compose.yml files have been **DISABLED** (replaced with empty stubs). The main orchestration now happens from `/home/tony/Documents/cloud-u-l8r/docker-compose.yml`.

## How to Deploy

### Option 1: Using Manual Command (Recommended)
```bash
cd /home/tony/Documents/cloud-u-l8r
docker-compose down    # Stop any running containers
docker-compose up -d   # Start all services
```

### Option 2: Using Provided Script
```bash
bash /home/tony/Documents/cloud-u-l8r/start-stack.sh
```

This script will:
- Automatically disable any rogue subdirectory compose files
- Stop running containers
- Deploy all services from the main docker-compose.yml
- Show status and available endpoints

## Service Configuration

The main `docker-compose.yml` defines four services:

### 1. **essthree** (S3 Emulator)
- Service name: `essthree`
- Container name: `essthree`
- Port: 9300
- Builds from: `./services/essthree`

### 2. **cloudfauxnt** (CloudFront Emulator)
- Service name: `cloudfauxnt`
- Container name: `cloudfauxnt`
- Port: 9310
- Builds from: `./services/cloudfauxnt`
- **Depends on:** essthree

### 3. **ess-queue-ess** (SQS Emulator)
- Service name: `ess-queue-ess`
- Container name: `ess-queue-ess`
- Port: 9320
- Builds from: `./services/ess-queue-ess`

### 4. **ess-enn-ess** (SNS Emulator)
- Service name: `ess-enn-ess`
- Container name: `ess-enn-ess`
- Ports: 9330 (API), 9331 (Admin Dashboard)
- Builds from: `./services/ess-enn-ess`
- **Depends on:** ess-queue-ess

## Verification

After deployment, verify all services are running:

```bash
docker-compose ps
```

Expected output:
```
NAME             COMMAND                  SERVICE        STATUS
essthree         "./ess-three"            essthree       Up <time>
cloudfauxnt      "./cloudfauxnt"          cloudfauxnt    Up <time>
ess-queue-ess    "./ess-queue-ess --co..." ess-queue-ess Up <time>
ess-enn-ess      "-config /app/config..." ess-enn-ess    Up <time>
```

Or test health endpoints:
```bash
# S3
curl http://localhost:9300/health

# CloudFront
curl http://localhost:9310/health

# SQS
curl http://localhost:9320/health

# SNS API
curl http://localhost:9330/health

# SNS Dashboard
curl http://localhost:9331/health
```

## Accessing Services

| Service | URL |
|---------|-----|
| S3 (essthree) | http://localhost:9300 |
| CloudFront (cloudfauxnt) | http://localhost:9310 |
| SQS (ess-queue-ess) | http://localhost:9320 |
| SNS API (ess-enn-ess) | http://localhost:9330 |
| SNS Admin Dashboard | http://localhost:9331 |

## Troubleshooting

### "Port already in use" error
```bash
# Find and stop conflicting containers
docker ps | grep -E "essthree|cloudfauxnt|ess-queue-ess|ess-enn-ess"
docker stop <container_id>

# Try again
docker-compose up -d
```

### "cloudfauxnt fails to start"
Ensure essthree is running first. Check logs:
```bash
docker-compose logs cloudfauxnt
```

### "ess-enn-ess fails to start"
Ensure ess-queue-ess is up. Check logs:
```bash
docker-compose logs ess-enn-ess
```

## Developer Notes

- **Always run `docker-compose` from the main `cloud-u-l8r` directory**
- Never run `docker-compose` from subdirectories (their compose files are now disabled)
- Services use a shared Docker network: `cloud-u-l8r_shared-network`
- All services restart automatically unless stopped: `restart: unless-stopped`
- Data is persisted in each service's `./data` directory

## What Changed

**Files Modified:**
1. `/services/essthree/docker-compose.yml` - Disabled (was conflicting)
2. `/services/ess-queue-ess/docker-compose.yml` - Disabled (was conflicting)
3. `/services/cloudfauxnt/docker-compose.yml` - Disabled (was conflicting)

**Files Created:**
1. `/start-stack.sh` - Convenient startup script
2. `/DOCKER_COMPOSE_FIX.md` - This file

**Main Orchestration File:**
- `/docker-compose.yml` - Unchanged, controls all four services
