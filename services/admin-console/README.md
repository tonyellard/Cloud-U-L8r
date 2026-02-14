# admin-console

Consolidated operator console for the local Cloud-U-L8r stack.

## Current Scope
- Dashboard across all services
- Full `ess-queue-ess` queue operations
- Full `ess-enn-ess` topic/subscription/publish operations
- `kay-vee` summary/activity/export plus parameter and secret workflows
- Informational `ess-three` summary (bucket/object overview)
- Informational `cloudfauxnt` summary (origin/behavior/signing overview)
- Active-view-only SSE updates

## Local Endpoints
- UI: `http://localhost:9999/`
- Health: `http://localhost:9999/health`
- Dashboard summary: `http://localhost:9999/api/dashboard/summary`
- SSE stream: `http://localhost:9999/api/events?view=dashboard`

## Key API Surfaces
- `GET /api/services/ess-queue-ess/queues`
- `GET /api/services/ess-queue-ess/queues/{queueID}/messages/peek`
- `GET /api/services/ess-queue-ess/queues/{queueID}/attributes`
- `POST /api/services/ess-queue-ess/actions/*` (create/send/update/purge/delete/redrive)
- `GET /api/services/ess-enn-ess/state`
- `GET /api/services/ess-enn-ess/topics/{topicARN}/activities`
- `POST /api/services/ess-enn-ess/actions/*` (create/delete topic, create/delete subscription, publish)
- `GET /api/services/kay-vee/summary`
- `GET /api/services/kay-vee/activity`
- `GET /api/services/kay-vee/export`
- `GET /api/services/kay-vee/parameters/by-path`
- `GET /api/services/kay-vee/secrets`
- `GET /api/services/kay-vee/secrets/value`
- `POST /api/services/kay-vee/actions/put-parameter`
- `POST /api/services/kay-vee/actions/delete-parameter`
- `POST /api/services/kay-vee/actions/label-parameter-version`
- `POST /api/services/kay-vee/actions/create-secret`
- `POST /api/services/kay-vee/actions/delete-secret`
- `POST /api/services/kay-vee/actions/restore-secret`
- `GET /api/services/essthree/summary`
- `GET /api/services/cloudfauxnt/summary`
- `GET /api/services/{service}/config/export` (`ess-queue-ess`, `ess-enn-ess`, and `kay-vee`)

## Internal Dependencies
- `http://ess-queue-ess:9320`
- `http://ess-enn-ess:9330`
- `http://kay-vee:9350`
- `http://essthree:9300`
- `http://cloudfauxnt:9310`

## Run
From repo root:

```bash
make up
```

Or build this service only:

```bash
docker compose build admin-console
```
