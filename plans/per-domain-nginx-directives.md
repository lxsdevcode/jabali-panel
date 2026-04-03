# Plan: Per-Domain Custom Nginx Directives

**Objective**: Let users add custom nginx directives per domain via a visual builder + raw textarea, with safe validation (nginx -t before applying).

## Design

Two-panel UI on the domain settings page (new "Advanced" tab):

1. **Builder** (left/top) — form-based, generates nginx config snippets
2. **Textarea** (right/bottom) — raw nginx directives, editable by advanced users

Builder output appends to the textarea. Users can use either or both.

## Builder Components

### Redirects
- Source path (`/old-page`)
- Destination URL (`https://example.com/new-page`)
- Type: 301 (permanent) / 302 (temporary)
- Generated: `rewrite ^/old-page$ https://example.com/new-page permanent;`

### Custom Headers
- Header name (dropdown: `X-Frame-Options`, `X-Content-Type-Options`, `Strict-Transport-Security`, `Content-Security-Policy`, custom)
- Value
- Generated: `add_header X-Frame-Options "DENY" always;`

### Rewrite Rules
- Match pattern (regex)
- Replacement
- Flags: last, break, redirect, permanent
- Generated: `rewrite ^/api/v1/(.*)$ /api/v2/$1 last;`

### IP Access Control
- Path (`/admin`, `/wp-admin`, `/` for whole site)
- Action: allow/deny
- IP/CIDR
- Generated: `location /admin { allow 1.2.3.4; deny all; }`

### Rate Limiting
- Path
- Rate (requests/second or requests/minute)
- Burst
- Generated: creates `limit_req_zone` + `limit_req` directives

### Proxy Pass
- Path (`/api`, `/app`)
- Upstream URL (`http://127.0.0.1:3000`)
- WebSocket support toggle
- Generated: `location /api { proxy_pass http://127.0.0.1:3000; proxy_set_header ... }`

### PHP Settings Override
- `memory_limit`, `upload_max_filesize`, `post_max_size`, `max_execution_time`, `max_input_vars`
- Generated: `fastcgi_param PHP_VALUE "memory_limit=512M\nupload_max_filesize=100M";`

## Data Model

### Migration
```php
// Add columns to domains table
$table->text('custom_nginx_directives')->nullable();
$table->json('custom_nginx_rules')->nullable(); // builder state for re-editing
```

### Agent Route
Existing `domain.update_vhost` route or new `domain.apply_custom_directives`:
1. Write directives to `/etc/nginx/jabali/custom/{domain}.conf`
2. Include file in domain's vhost: `include /etc/nginx/jabali/custom/{domain}.conf;`
3. Run `nginx -t`
4. If pass → reload nginx, save to DB
5. If fail → delete file, return error

## Validation

### Directive Allowlist (user panel)
Users can only use these directives:
- `rewrite`, `return`, `try_files`
- `add_header`
- `location` (limited: no `alias`, no `root` override)
- `proxy_pass`, `proxy_set_header`, `proxy_http_version`
- `limit_req`, `limit_req_zone`
- `allow`, `deny`
- `fastcgi_param PHP_VALUE`, `fastcgi_param PHP_ADMIN_VALUE`
- `client_max_body_size`
- `expires`, `access_log off`

### Blocked Directives (always rejected)
- `include` (path traversal risk)
- `alias` (path traversal risk)
- `root` (override document root)
- `lua_`, `content_by_lua`, `access_by_lua` (code execution)
- `load_module` (arbitrary module loading)
- `ssl_certificate`, `ssl_certificate_key` (cert hijack)
- `error_log`, `access_log` with absolute paths outside user dir

### Admin panel
No restrictions — admins can write any directive.

### nginx -t gate
Regardless of allowlist, config is always tested before applying. This is the final safety net.

## File Layout

```
/etc/nginx/jabali/custom/
    example.com.conf     # custom directives for example.com
    other-site.com.conf  # custom directives for other-site.com
```

Domain vhost includes it:
```nginx
server {
    ...
    include /etc/nginx/jabali/custom/example.com.conf;
    ...
}
```

Empty file = no custom directives. Missing file = skip (nginx handles missing includes gracefully with wildcard).

## UI Flow

### Filament Page
New tab "Advanced" on domain settings (both admin and user panels).

**Builder section:**
- Repeatable form rows, each with a type selector (redirect, header, rewrite, etc.)
- Each type expands to its specific fields
- "Add Rule" button
- Rules can be reordered, edited, deleted

**Textarea section:**
- Below the builder
- Shows the generated config + any manually added directives
- "Generate from rules" button syncs builder → textarea
- Manual edits in textarea are preserved (builder state stored separately in `custom_nginx_rules` JSON)

**Save button:**
1. Sends directives to agent
2. Agent writes temp file, runs nginx -t
3. On success: saves to DB, reloads nginx, shows success notification
4. On failure: shows nginx error in a modal, no changes applied

## Steps

### Step 1: Migration + Model
- Add `custom_nginx_directives` (text) and `custom_nginx_rules` (json) to domains table
- Update Domain model with casts

### Step 2: Agent Route
- New `domain.apply_custom_directives` in jabali-agent
- Writes to `/etc/nginx/jabali/custom/{domain}.conf`
- Runs nginx -t, rolls back on failure
- Returns error output on failure for display

### Step 3: Vhost Template Update
- Add `include /etc/nginx/jabali/custom/*.conf;` pattern or per-domain include to agent's vhost generator

### Step 4: Directive Validator
- `App\Services\NginxDirectiveValidator` class
- Allowlist check for user panel, passthrough for admin
- Regex-based directive extraction and validation

### Step 5: Builder UI (Livewire)
- Repeatable form component with type-specific sub-forms
- Generates nginx config string from structured rules
- Stores structured rules in `custom_nginx_rules` JSON

### Step 6: Advanced Tab
- Add to both admin and user domain pages
- Builder + textarea + save button
- Error display modal for nginx -t failures

## Security Notes

- Directives validated against allowlist before reaching the agent
- Agent runs nginx -t as final gate — even if allowlist is bypassed, bad config won't apply
- User directives scoped to their domain's server block only
- No `include`, `alias`, `root` to prevent path traversal
- Admin panel bypasses allowlist (full control)
- All changes audit logged
