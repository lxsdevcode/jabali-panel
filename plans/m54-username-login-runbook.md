# M54 Username-login — operator cutover runbook

Fresh installs are username-login from first boot (install ships the v2 schema;
the bootstrap admin gets a derived username). **Existing installs** need a
one-time cutover after `jabali update`, because the deployed Kratos schema is
NOT auto-swapped (that would lock users out before the re-key).

On `jabali update`, migration 000164 runs automatically: it backfills any NULL
username (ULID fallback), drops the email-unique index, and makes username NOT
NULL. The DB is then username-keyed, but Kratos still logs in by email until you
run the steps below. This is safe (no lockout) — just complete it to activate
duplicate-email support + username login.

Cutover (validated on 10.0.3.14 2026-06-10):

```bash
# 1. give every user a friendly username (email-derived). Idempotent.
jabali admin backfill-usernames            # dry-run: review the plan
jabali admin backfill-usernames --apply

# 2. deploy the username-identifier schema + reload Kratos
install -m0644 /opt/jabali-panel/install/kratos-identity-schema.json \
  /etc/jabali-panel/kratos-identity-schema.json
systemctl restart jabali-kratos.service

# 3. re-key every Kratos identity email -> username
jabali admin relabel-identifiers           # dry-run
jabali admin relabel-identifiers --apply   # 0 failed expected

# 4. verify: identities now key on username
#    curl --unix-socket /run/jabali-kratos/admin.sock \
#      "http://localhost/admin/identities/<id>?include_credential=password"
#    -> credentials.password.identifiers == ["<username>"]
```

Users now log in by **username**. Email is a plain contact (may be shared, not
used for login). There is **no self-service password recovery** — an admin
resets via the user row → **Reset password** (reveal-once temp password).

Rollback (before step 3 completes, if needed): restore the previous schema
(`/etc/jabali-panel/kratos-identity-schema.json.m54bak` if you backed it up) and
restart Kratos — identities keep their email identifier.
