# Integration Tests

This directory contains integration tests for the Cloud-U-L8r monorepo.

## Test Scripts

### test_cross_service.sh

Comprehensive integration test that verifies:

1. **Service Health Checks**
   - essthree (S3 emulator) on port 9300
   - cloudfauxnt (CloudFront emulator) on port 9310
   - ess-queue-ess (SQS emulator) on port 9320

2. **Cross-Service Communication**
   - CloudFauxnt → essthree: Verifies CloudFront can proxy requests to S3
   - Application → ess-queue-ess: Verifies SQS queue operations work

3. **Data Flow Verification**
   - Creates test files in essthree
   - Accesses them directly and through CloudFauxnt
   - Creates SQS queues and verifies operations

## Running Tests

### Prerequisites

Services must be running:
```bash
cd /home/tony/Documents/cloud-u-l8r
make up
```

### Run All Integration Tests

```bash
./tests/integration/test_cross_service.sh
```

### Expected Output

```
=== Cloud-U-L8r Integration Test ===

1. Testing essthree (S3) health endpoint...
✓ essthree is healthy
2. Testing cloudfauxnt (CloudFront) endpoint...
✓ cloudfauxnt is responding
3. Testing ess-queue-ess (SQS) admin endpoint...
✓ ess-queue-ess is responding
4. Testing cross-service communication (CloudFauxnt -> essthree)...
✓ Direct access to essthree works
✓ Access via CloudFauxnt works (cross-service communication confirmed)
5. Testing ess-queue-ess queue operations...
✓ Queue operations working

=== All Integration Tests Passed ===
```

## Test Coverage

Current integration tests verify:
- ✅ All three services are running and healthy
- ✅ CloudFauxnt can successfully proxy requests to essthree
- ✅ SQS queue creation and basic operations work
- ✅ Cross-service networking is configured correctly

## Adding New Tests

When adding new integration tests:

1. Create a new script in this directory
2. Follow the naming convention: `test_*.sh`
3. Make it executable: `chmod +x test_*.sh`
4. Add appropriate error handling (`set -e`)
5. Use colored output for clarity (GREEN for success, RED for failure)
6. Document what the test verifies
7. Add it to this README

## Debugging Failed Tests

If a test fails:

1. **Check service status**: `docker ps`
2. **Check logs**: `make logs` or `docker logs <service-name>`
3. **Verify ports are accessible**: `curl -v http://localhost:<port>/`
4. **Check network**: `docker network inspect cloud-u-l8r_shared-network`
5. **Restart services**: `make down && make up`

## CI/CD Integration

These tests can be integrated into CI/CD pipelines:

```bash
#!/bin/bash
set -e

# Start services
make up

# Wait for services to be ready
sleep 5

# Run tests
./tests/integration/test_cross_service.sh

# Cleanup
make down
```
