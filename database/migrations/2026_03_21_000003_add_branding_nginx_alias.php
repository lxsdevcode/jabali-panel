<?php

declare(strict_types=1);

use Illuminate\Database\Migrations\Migration;
use Illuminate\Support\Facades\File;
use Illuminate\Support\Facades\Log;
use Symfony\Component\Process\Process;

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
            $test = new Process(['nginx', '-t']);
            $test->run();

            if ($test->isSuccessful()) {
                (new Process(['systemctl', 'reload', 'nginx']))->run();
                Log::info('Migration: Branding nginx alias added to vhosts');
            } else {
                Log::warning('Migration: Nginx config test failed after adding branding alias');
            }
        }
    }

    public function down(): void
    {
        // Removing nginx location blocks automatically is risky
    }
};
