<?php

return [
    'demo' => env('JABALI_DEMO', false),

    'agent' => [
        'socket' => env('JABALI_AGENT_SOCKET', '/var/run/jabali/agent.sock'),
        'timeout' => env('JABALI_AGENT_TIMEOUT', 30),
    ],

    'mail_backend' => env('MAIL_BACKEND', 'legacy'),

    'panel' => [
        'port' => (int) env('PANEL_PORT', 2223),
        'hostname' => env('PANEL_HOSTNAME', env('SERVER_HOSTNAME', '')),
        'cert_storage' => env('PANEL_CERT_STORAGE', '/var/lib/jabali/caddy'),
        'acme_email' => env('PANEL_ACME_EMAIL', ''),
        'caddyfile' => env('PANEL_CADDYFILE', '/etc/jabali/Caddyfile'),
    ],
];
