<?php

declare(strict_types=1);

use App\Models\DnsSetting;
use Illuminate\Database\Migrations\Migration;
use Illuminate\Support\Facades\File;
use Illuminate\Support\Facades\Log;
use Symfony\Component\Process\Process;

return new class extends Migration
{
    public function up(): void
    {
        if (config('jabali.mail_backend') !== 'stalwart') {
            return;
        }

        $configPath = '/etc/stalwart-mail/config.toml';
        if (! File::exists($configPath)) {
            return;
        }

        $config = File::get($configPath);
        $hostname = DnsSetting::get('hostname', gethostname() ?: 'localhost');

        // Find the best LE cert available
        $certDomain = null;
        foreach (["mail.{$hostname}", $hostname] as $candidate) {
            if (File::exists("/etc/letsencrypt/live/{$candidate}/fullchain.pem")) {
                $certDomain = $candidate;
                break;
            }
        }

        if (! $certDomain) {
            return;
        }

        $certPath = "/etc/letsencrypt/live/{$certDomain}/fullchain.pem";
        $keyPath = "/etc/letsencrypt/live/{$certDomain}/privkey.pem";
        $expectedCert = "%{file:{$certPath}}%";

        // Already correct
        if (str_contains($config, $expectedCert)) {
            return;
        }

        $expectedKey = "%{file:{$keyPath}}%";
        $config = preg_replace(
            '/\[certificate\.default\]\s*\n(?:(?:cert|private-key|default)\s*=\s*[^\n]+\n?)+/',
            "[certificate.default]\ncert = \"{$expectedCert}\"\nprivate-key = \"{$expectedKey}\"\ndefault = true\n",
            $config
        );

        File::put($configPath, $config);
        (new Process(['systemctl', 'restart', 'stalwart-mail']))->setTimeout(30)->run();
        Log::info("Migration: Stalwart TLS updated to use LE cert for {$certDomain}");
    }

    public function down(): void
    {
        // TLS cert changes are not reversible in a meaningful way
    }
};
