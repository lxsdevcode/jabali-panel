<?php

declare(strict_types=1);

use App\Services\Agent\AgentClient;
use Illuminate\Database\Migrations\Migration;
use Illuminate\Support\Facades\File;
use Illuminate\Support\Facades\Log;

return new class extends Migration
{
    public function up(): void
    {
        if (! File::isDirectory('/etc/nginx/sites-enabled')) {
            return;
        }

        $brandingBlock = "    location /branding/ {\n        alias /opt/bulwark/public/branding/;\n        expires 7d;\n        access_log off;\n    }\n";
        $modified = false;

        foreach (File::glob('/etc/nginx/sites-enabled/*.conf') as $conf) {
            if (! is_writable($conf)) {
                continue;
            }

            $content = File::get($conf);

            if (str_contains($content, 'location /branding/') || ! str_contains($content, 'location = /webmail')) {
                continue;
            }

            $content = str_replace(
                '    location = /webmail {',
                "{$brandingBlock}\n    location = /webmail {",
                $content
            );

            File::put($conf, $content);
            $modified = true;
        }

        if ($modified) {
            try {
                $agent = app(AgentClient::class);
                $agent->send('service.reload', ['service' => 'nginx']);
                Log::info('Migration: Branding nginx alias added to vhosts');
            } catch (Exception $e) {
                Log::warning("Migration: Could not reload nginx: {$e->getMessage()}");
            }
        }
    }

    public function down(): void
    {
        // Removing nginx location blocks automatically is risky
    }
};
