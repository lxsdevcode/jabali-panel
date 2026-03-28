<?php

return [
    'demo' => env('JABALI_DEMO', false),

    'agent' => [
        'socket' => env('JABALI_AGENT_SOCKET', '/var/run/jabali/agent.sock'),
        'timeout' => env('JABALI_AGENT_TIMEOUT', 30),
    ],

    'mail_backend' => env('MAIL_BACKEND', 'legacy'),

    'backup' => [
        'default_repo' => env('JABALI_BACKUP_REPO', '/var/backups/jabali/restic'),
    ],

    'panel' => [
        'port' => (int) env('PANEL_PORT', 2223),
        'hostname' => env('PANEL_HOSTNAME', env('SERVER_HOSTNAME', '')),
        'tls_cert' => env('PANEL_TLS_CERT', '/etc/ssl/jabali/panel.crt'),
        'tls_key' => env('PANEL_TLS_KEY', '/etc/ssl/jabali/panel.key'),
        'caddyfile' => env('PANEL_CADDYFILE', '/etc/jabali/Caddyfile'),
    ],
];
