<?php

declare(strict_types=1);

use App\Models\DnsSetting;
use App\Services\Agent\AgentClient;
use Exception;
use Illuminate\Database\Migrations\Migration;
use Illuminate\Support\Facades\File;
use Illuminate\Support\Facades\Log;

return new class extends Migration
{
    public function up(): void
    {
        if (config('jabali.mail_backend') !== 'stalwart') {
            return;
        }

        $configPath = '/etc/stalwart-mail/config.toml';
        if (! File::exists($configPath) || ! is_readable($configPath)) {
            return;
        }

        $config = File::get($configPath);
        $hostname = DnsSetting::get('hostname', gethostname() ?: 'localhost');

        // Find the best LE cert available
        // Try: mail.hostname, then extract root domain and try mail.rootdomain, then hostname itself
        $candidates = ["mail.{$hostname}"];
        $parts = explode('.', $hostname);
        if (count($parts) > 2) {
            $rootDomain = implode('.', array_slice($parts, -2));
            $candidates[] = "mail.{$rootDomain}";
        }
        $candidates[] = $hostname;

        $certDomain = null;
        foreach ($candidates as $candidate) {
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

        // Can't write as www-data — skip silently, postinst runs as root
        if (! is_writable($configPath)) {
            Log::info('Migration: Stalwart TLS fix skipped (no write permission), will apply on next root upgrade');

            return;
        }

        $expectedKey = "%{file:{$keyPath}}%";
        $config = preg_replace(
            '/\[certificate\.default\]\s*\n(?:(?:cert|private-key|default)\s*=\s*[^\n]+\n?)+/',
            "[certificate.default]\ncert = \"{$expectedCert}\"\nprivate-key = \"{$expectedKey}\"\ndefault = true\n",
            $config
        );

        File::put($configPath, $config);

        try {
            $agent = app(AgentClient::class);
            $agent->send('service.restart', ['service' => 'stalwart-mail']);
        } catch (Exception $e) {
            Log::warning("Migration: Could not restart Stalwart: {$e->getMessage()}");
        }

        Log::info("Migration: Stalwart TLS updated to use LE cert for {$certDomain}");
    }

    public function down(): void
    {
        // TLS cert changes are not reversible in a meaningful way
    }
};
