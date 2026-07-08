# Platform — Health Monitor

The same surface as [Server Status](../server-status.md) but exposed at machine-readable endpoints for external monitoring.

## Endpoints

| Endpoint | Auth | Returns |
|---|---|---|
| `GET /api/v1/health` | none | `{ "status": "ok" \| "degraded" \| "down", "version": "...", "uptime_s": N }` — used as the basic liveness probe. |
| `GET /api/v1/health/detailed` | admin Bearer / cookie | Per-service status (same as `/jabali-admin/server-status`) as JSON. |
| `GET /metrics` | admin Bearer | Prometheus-format metrics (request counts, latencies, reconciler tick durations, queue depths). |
| `GET /api/v1/automation/status` | Automation token, scope `read:status` | Full server metrics for an external fleet monitor (see below). |

## Fleet metrics — `/api/v1/automation/status`

For a multi-server manager that polls each Jabali server without a panel session
(GH #308 / JAB-75). Authenticated with an **automation token** (HMAC, replay-defended)
carrying the `read:status` scope — mint one with:

```
jabali automation-token create --scope read:status
```

Returns the same collectors as the admin Server Status page (one source of truth),
gathered from the agent in parallel and cached ~5 s so frequent polling doesn't
hammer the agent:

```json
{
  "healthy": true,
  "time": "2026-07-08T20:10:00Z",
  "version": "<panel build sha>",
  "system":   { "hostname": "...", "uptime_seconds": N, "load_avg": [..],
                "cpu_count": N, "mem_total_kb": N, "mem_used_kb": N,
                "swap_total_kb": N, "partitions": [{ "mount_point": "/",
                "total_bytes": N, "used_bytes": N, "free_bytes": N }] },
  "services": { "services": [ { "unit": "...", "load_state": "...", "active_state": "..." } ] },
  "cpu":      { ... live CPU usage ... },
  "errors":   { "info": "timeout" }   // only present when a collector failed
}
```

- `healthy` is `true` when every collector returned; a partial failure surfaces
  per-slot in `errors` but still returns HTTP 200 with whatever was collected.
- The payload is **metrics only** — no credentials, tokens, or per-tenant infra.

## Status semantics

| status | Meaning |
|---|---|
| `ok` | All watched services healthy. |
| `degraded` | A non-critical service is failed/degraded (e.g. ClamAV freshclam stale > 7 d). Panel still serves. |
| `down` | A critical service is failed (panel-api itself returning health, so MariaDB / nginx / Stalwart down counts here). |

## Watched services

- `jabali-panel.service`
- `jabali-agent.service`
- `nginx.service`
- `mariadb.service`
- `postgresql.service` (only if at least one user has a Postgres DB; otherwise ignored)
- `pdns.service`
- `pdns-recursor.service`
- `stalwart-mail.service`
- `kratos.service`
- `bulwark.service`
- `redis.service`
- `crowdsec.service`

The watched set is computed at startup; services that aren't installed don't count against `degraded`.

## Use with an external monitor

UptimeRobot / Pingdom / a self-hosted Uptime-Kuma:

- Point at `https://<panel-hostname>/api/v1/health` — anonymous, fast.
- Set expected response: HTTP 200 + body contains `"status":"ok"`.

For Prometheus scraping, point at `/metrics` with a bearer token (mint under `/jabali-admin/automation`).

## Notifications integration

The `service_down` event source (M14) reads the same internal state — so you don't *need* an external monitor for in-house alerting. The external monitor is useful for "the panel-api itself is down, who tells me?" cases.
