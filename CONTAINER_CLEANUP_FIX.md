# Docker Compose Container Cleanup Issue - Root Cause & Fix

## The Problem

When running `make down`, only 3 containers were being stopped:
- cloudfauxnt ✅
- essthree ✅
- Network ✅

But these remained running:
- ess-queue-ess ❌
- ess-enn-ess ❌
- ess-three ❌ (Note: different naming - "ess-three" vs "essthree")

## Root Cause

The issue stems from **conflicting docker-compose files and orphaned containers**:

### 1. **Stale docker-compose.yml Files**
Each subdirectory had its own docker-compose.yml that defined containers with different names:
- `services/essthree/docker-compose.yml` defined a service called `ess-three` (not `essthree`)
- These files were disabled, but their containers remained in Docker

### 2. **Container Naming Mismatch**
The main `docker-compose.yml` uses:
- `container_name: essthree` (service name: `essthree`)
- `container_name: ess-queue-ess`
- `container_name: ess-enn-ess`

But the old subdirectory files created:
- `container_name: ess-three` (different name!)
- `container_name: ess-queue-ess` (same)
- No explicit container for ess-enn-ess in essthree's compose

### 3. **docker compose down Limitation**
`docker compose down` only removes containers explicitly defined in the active docker-compose.yml. Orphaned containers from old/disabled compose files are NOT removed.

## Solutions Implemented

### 1. **Updated Makefile `down` Target**
```makefile
down:
	@echo "Stopping services..."
	docker compose down -v
	@echo "Removing stray containers..."
	docker rm -f essthree ess-three cloudfauxnt ess-queue-ess ess-enn-ess 2>/dev/null || true
	@echo "✅ All services stopped and cleaned up
```

This now:
- Stops containers from docker-compose.yml
- Force-removes any stray containers by name (handles both old and new naming)
- Silently ignores containers that don't exist

### 2. **Updated Makefile `clean` Target**
```makefile
clean:
	@echo "Cleaning up all Docker artifacts..."
	docker compose down -v --rmi local 2>/dev/null || true
	@echo "Removing stray containers..."
	docker rm -f essthree ess-three cloudfauxnt ess-queue-ess ess-enn-ess 2>/dev/null || true
	@echo "Removing stray volumes..."
	docker volume rm cloud-u-l8r_shared-network 2>/dev/null || true
	@echo "✅ Cleanup complete
```

### 3. **Created cleanup-stack.sh Script**
For manual cleanup when make isn't available:
```bash
bash /home/tony/Documents/cloud-u-l8r/cleanup-stack.sh
```

## How to Verify the Fix

### Test 1: Cleanup Everything
```bash
cd /home/tony/Documents/cloud-u-l8r
make down
docker ps -a  # Should show NO emulator containers
```

### Test 2: Fresh Deploy
```bash
make up
docker ps    # Should show all 4 containers running
```

### Test 3: Complete Clean
```bash
make clean
docker ps -a  # Should show NO emulator containers
docker images | grep cloud-u-l8r  # Should show NO emulator images
```

## Prevention for Future Issues

### Best Practices Going Forward

1. **Always run docker-compose from the root directory:**
   ```bash
   cd /home/tony/Documents/cloud-u-l8r
   docker-compose up -d
   ```

2. **Never run docker-compose from subdirectories:**
   ```bash
   # ❌ DON'T DO THIS:
   cd services/essthree
   docker-compose up -d
   ```

3. **Keep subdirectory compose files disabled:**
   They now contain empty stubs with comments explaining they're disabled

4. **Use make targets for consistency:**
   ```bash
   make up      # Start all services
   make down    # Stop all services
   make clean   # Remove all artifacts
   ```

## Container Name Reference

| Service | Container Name | Port | Status |
|---------|---|---|---|
| S3 | `essthree` | 9300 | Main compose file |
| CloudFront | `cloudfauxnt` | 9310 | Main compose file |
| SQS | `ess-queue-ess` | 9320 | Main compose file |
| SNS | `ess-enn-ess` | 9330-9331 | Main compose file |
| **OLD (orphaned)** | `ess-three` | 9300 | From disabled compose ❌ |

## Files Modified

1. **Makefile** - Updated `down` and `clean` targets
2. **cleanup-stack.sh** (NEW) - Manual cleanup script
3. **start-stack.sh** (existing) - Uses main docker-compose.yml
4. **verify-stack.sh** (existing) - Health checks for services

## Quick Reference

```bash
# Start everything fresh
make down && make up && sleep 5 && make verify

# Clean everything
make clean

# Manual cleanup if needed
bash cleanup-stack.sh
```

## Troubleshooting

### Still seeing stray containers after `make down`?
```bash
# List all emulator containers
docker ps -a | grep -E "essthree|ess-three|ess-queue-ess|ess-enn-ess|cloudfauxnt"

# Force remove specific container
docker rm -f <container_name>
```

### "docker compose down -v" failing?
```bash
# Manually stop and remove
docker stop essthree ess-queue-ess ess-enn-ess cloudfauxnt 2>/dev/null || true
docker rm essthree ess-queue-ess ess-enn-ess cloudfauxnt 2>/dev/null || true
```

### Network already exists?
```bash
# Remove the network
docker network rm cloud-u-l8r_shared-network

# Recreate it
make up
```

## Summary

The updated Makefile now properly cleans up all containers, both those defined in the main docker-compose.yml AND orphaned containers from old/disabled compose files. This ensures a clean state every time you run `make down` or `make clean`.
