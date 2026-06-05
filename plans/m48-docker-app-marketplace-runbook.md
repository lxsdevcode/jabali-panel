# M48 Docker App Marketplace — Operator Runbook

Companion to [`plans/m48-docker-app-marketplace.md`](m48-docker-app-marketplace.md) and [ADR-0116](../docs/adr/0116-m48-docker-app-marketplace.md). For when something is on fire on a live host.

---

## 1. What's installed

**Files**

```
/etc/docker/daemon.json                          our drop-in (live-restore + journald + ulimits)
/usr/local/share/jabali/docker-apps/<slug>/      catalog (synced by `jabali update`)
/var/lib/jabali/docker-apps/<slug>/              per-install data root (one per docker_apps row)
    compose.yml                                    rendered from catalog template
    .env                                           secrets (mode 0600)
    config/  data/  db/  uploads/  secrets/        catalog-declared bind-mounts
    .jabali-meta.json                              install metadata sidecar
```

**Database**

```sql
docker_apps                       -- one row per install
docker_app_published_ports        -- ports per install (multi-row)
docker_app_backups                -- restic snapshot tracker
domains                           -- managed_by='docker_app' + docker_app_id when nginx proxy is wired
```

**Agent verbs**

```
docker_app.install   docker_app.start   docker_app.stop
docker_app.restart   docker_app.rebuild docker_app.delete
docker_app.status    docker_app.update
```

---

## 2. Health check from the shell

```bash
# Are all installed apps in `running` state?
mysql -uroot jabali_panel -e "
  SELECT slug, name, status, last_error
  FROM docker_apps
  WHERE status NOT IN ('running','stopped');
"

# What does docker say?
for d in /var/lib/jabali/docker-apps/*/; do
  echo "=== $d ==="
  (cd "$d" && docker compose ps)
done
```

If `docker_apps.status` and `docker compose ps` disagree, the reconciler hasn't run a status-poll yet (worst case ~60s).

---

## 3. Install path debug

Symptoms: `POST /admin/docker-apps` returns 502 `agent_install_failed`.

```bash
# Latest agent invocations + their stderr blobs.
journalctl -u jabali-agent --since='10 min ago' | grep docker_app

# Look at the rendered compose if the agent wrote it.
ls /var/lib/jabali/docker-apps/<slug>/
cat /var/lib/jabali/docker-apps/<slug>/compose.yml

# Try the compose directly.
cd /var/lib/jabali/docker-apps/<slug> && docker compose up
```

Most install failures fall into:

| Symptom | Cause | Fix |
|---|---|---|
| `port is already allocated` | host_port collides with a non-docker service | re-install with a different `host_port` in the request body, or stop the other service |
| `pull access denied` | catalog points at a private image | edit catalog `image_channel` to a public mirror |
| healthcheck times out | upstream `wait_healthy` budget too tight | re-install; agent default is 120s, can be bumped via `healthcheck_timeout_seconds` |
| `manifest unknown` | catalog channel tag was deleted upstream | pin a specific version in the catalog |

---

## 4. Update path debug

`POST /admin/docker-apps/:id/update` outcomes:

| `outcome` | What happened | Recover by |
|---|---|---|
| `updated` | new image is healthy and running | nothing |
| `rolled_back` | pull or healthcheck failed; previous compose still running | check `last_error`; pin a different image channel in catalog and retry |

Update path:

```
1. restic snapshot /var/lib/jabali/docker-apps/<slug>  (when restic configured)
2. docker compose pull
3. docker compose up -d
4. wait for healthy (default 120s, override via payload)
5. on fail -> docker compose up -d on the old digest (restic restore in Phase 8)
```

---

## 5. Manual disaster-recovery

Reinstall from the catalog without losing data:

```bash
mysql -uroot jabali_panel -e "
  UPDATE docker_apps SET status='pending', last_error=NULL
  WHERE slug='<slug>' AND name='<name>';
"
# Reconciler picks it up within 60s. Recovery dispatch reads
# compose.yml from disk and brings it back up.
```

Nuke a stuck install:

```bash
# 1. Stop the container set.
cd /var/lib/jabali/docker-apps/<slug> && docker compose down -v

# 2. Drop the panel-side rows.
mysql -uroot jabali_panel -e "
  DELETE FROM docker_apps WHERE slug='<slug>' AND name='<name>';
  -- cascades docker_app_published_ports + docker_app_backups
"

# 3. Manually drop the docker-app-managed domain row, if any.
mysql -uroot jabali_panel -e "
  DELETE FROM domains
  WHERE managed_by='docker_app' AND docker_app_id IS NULL;
"

# 4. Remove data root.
trash /var/lib/jabali/docker-apps/<slug>
```

---

## 6. Catalog hygiene

Add a new app:

```bash
mkdir -p install/docker-apps/<slug>
$EDITOR install/docker-apps/<slug>/app.yaml
$EDITOR install/docker-apps/<slug>/compose.yml.tmpl
cp ~/icon.svg install/docker-apps/<slug>/

# Verify the entry parses + matches the schema.
cd panel-api
go test ./internal/dockerapp/... -run TestCatalogLoad_RealCatalogParsesCleanly -v
```

Test the rendered compose against a known input:

```bash
go test ./internal/dockerapp/... -run TestRender_VaultwardenSingleHTTPPort
```

A new app ships when the test passes AND a smoke-install on a staging VM completes the lifecycle: install -> start -> stop -> restart -> update -> uninstall.

---

## 7. Known limitations (v1)

- **Admin-only.** Tenants can't install. Per-tenant docker is queued.
- **Bundled DB only.** Catalog apps ship their own DB inside the compose project. The hybrid mode (use jabali MariaDB) lands in M48.x.
- **Update rollback is best-effort.** Phase 7 ships compose-level rollback; restic-restore-based rollback is Phase 8.
- **Backup is manual.** Phase 8 wires the per-app backup REST endpoint into the operator's existing restic destination.
- **Exec shell + logs** UI not yet wired (the agent has the verbs).
- **Auto-update poller** not yet scheduled (the agent has the update verb; nothing polls upstream digests yet).
