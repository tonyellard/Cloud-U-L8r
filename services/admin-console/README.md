# admin-console

Consolidated operator console service for the local Cloud-U-L8r stack.

## v1 Scope
- Dashboard view
- ess-queue-ess service view
- Active-view-only SSE updates

## Local Endpoints
- UI: `http://localhost:9340/`
- Health: `http://localhost:9340/health`
- Dashboard API: `http://localhost:9340/api/dashboard/summary`
- Queue API: `http://localhost:9340/api/services/ess-queue-ess/queues`
- SSE: `http://localhost:9340/api/events?view=dashboard`

## Internal Dependencies
- `http://ess-queue-ess:9320`
- `http://ess-enn-ess:9330`

## Run
From repo root:

```bash
make up
```

Or build this service only:

```bash
docker compose build admin-console
```
