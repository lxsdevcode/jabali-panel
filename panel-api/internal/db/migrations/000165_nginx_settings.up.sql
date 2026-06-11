-- M55 Nginx Settings tab. Server-wide nginx tunables surfaced in
-- Server Settings → Nginx and rendered by the agent's nginx.tunables.apply
-- verb into /etc/nginx/conf.d/05-jabali-tunables.conf (http scope), with
-- the two worker-scope knobs patched into /etc/nginx/nginx.conf.
--
-- All columns NOT NULL with defaults matching install.sh's seeded fragment,
-- so existing installs see identical behaviour until an admin edits them.
-- Timeout/size columns are VARCHAR holding the nginx value verbatim (e.g.
-- "50m", "300s"); validated at the API + agent boundary by regex.
ALTER TABLE server_settings
  ADD COLUMN nginx_client_max_body_size  VARCHAR(16)  NOT NULL DEFAULT '50m',
  ADD COLUMN nginx_keepalive_timeout     VARCHAR(16)  NOT NULL DEFAULT '65s',
  ADD COLUMN nginx_server_tokens         TINYINT(1)   NOT NULL DEFAULT 0,
  ADD COLUMN nginx_gzip                  TINYINT(1)   NOT NULL DEFAULT 1,
  ADD COLUMN nginx_client_body_timeout   VARCHAR(16)  NOT NULL DEFAULT '60s',
  ADD COLUMN nginx_client_header_timeout VARCHAR(16)  NOT NULL DEFAULT '60s',
  ADD COLUMN nginx_send_timeout          VARCHAR(16)  NOT NULL DEFAULT '60s',
  ADD COLUMN nginx_proxy_connect_timeout VARCHAR(16)  NOT NULL DEFAULT '300s',
  ADD COLUMN nginx_proxy_read_timeout    VARCHAR(16)  NOT NULL DEFAULT '300s',
  ADD COLUMN nginx_proxy_send_timeout    VARCHAR(16)  NOT NULL DEFAULT '300s',
  ADD COLUMN nginx_worker_processes      VARCHAR(8)   NOT NULL DEFAULT 'auto',
  ADD COLUMN nginx_worker_connections    INT UNSIGNED NOT NULL DEFAULT 768,
  ADD COLUMN nginx_custom_http           VARCHAR(4000) NOT NULL DEFAULT '';
