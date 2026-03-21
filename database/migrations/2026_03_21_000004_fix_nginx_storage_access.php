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

        $modified = false;

        foreach (File::glob('/etc/nginx/sites-enabled/*') as $conf) {
            if (! is_writable($conf)) {
                continue;
            }

            $content = File::get($conf);

            // Remove |storage from the deny block: (vendor|node_modules|storage) -> (vendor|node_modules)
            if (str_contains($content, 'vendor|node_modules|storage')) {
                $content = str_replace('vendor|node_modules|storage', 'vendor|node_modules', $content);
                File::put($conf, $content);
                $modified = true;
            }
        }

        // Also fix sites-available
        foreach (File::glob('/etc/nginx/sites-available/*') as $conf) {
            if (! is_writable($conf)) {
                continue;
            }

            $content = File::get($conf);

            if (str_contains($content, 'vendor|node_modules|storage')) {
                $content = str_replace('vendor|node_modules|storage', 'vendor|node_modules', $content);
                File::put($conf, $content);
            }
        }

        if ($modified) {
            $test = new Process(['nginx', '-t']);
            $test->run();

            if ($test->isSuccessful()) {
                (new Process(['systemctl', 'reload', 'nginx']))->run();
                Log::info('Migration: Removed /storage/ from nginx deny blocks');
            } else {
                Log::warning('Migration: Nginx config test failed after fixing storage access');
            }
        }
    }

    public function down(): void
    {
        // Not safe to re-add the block automatically
    }
};
