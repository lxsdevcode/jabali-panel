<?php

declare(strict_types=1);

use App\Models\DnsSetting;
use App\Models\Domain;
use App\Models\EmailDomain;
use App\Models\Mailbox;
use App\Models\User;
use App\Services\Agent\AgentClient;
use App\Services\System\MailRoutingSyncService;
use Illuminate\Database\Migrations\Migration;
use Illuminate\Support\Facades\Crypt;
use Illuminate\Support\Facades\File;
use Illuminate\Support\Facades\Log;
use Illuminate\Support\Str;

/**
 * Combined system migration for Stalwart mail backend servers:
 * 1. Fix Stalwart TLS cert to use LE cert with %{file:...}% macro
 * 2. Create webmaster mailbox and configure authenticated SMTP
 * 3. Add /branding/ nginx alias for Bulwark webmail assets
 * 4. Remove /storage/ from nginx deny blocks (logo uploads)
 *
 * All operations are idempotent and use the agent for root operations.
 */
return new class extends Migration
{
    public function up(): void
    {
        if (config('jabali.mail_backend') !== 'stalwart') {
            return;
        }

        $hostname = DnsSetting::get('hostname', gethostname() ?: 'localhost');

        $this->fixStalwartTls($hostname);
        $password = $this->ensureWebmasterMailbox($hostname);
        $this->configureSmtp($hostname, $password);
        $this->fixNginxConfigs();
    }

    private function fixStalwartTls(string $hostname): void
    {
        $configPath = '/etc/stalwart-mail/config.toml';
        if (! File::exists($configPath) || ! is_readable($configPath)) {
            return;
        }

        $config = File::get($configPath);

        // Find the best LE cert: mail.rootdomain > mail.hostname > hostname
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

        $expectedCert = "%{file:/etc/letsencrypt/live/{$certDomain}/fullchain.pem}%";

        if (str_contains($config, $expectedCert)) {
            return;
        }

        if (! is_writable($configPath)) {
            // Try via agent
            try {
                $agent = app(AgentClient::class);
                $agent->send('file.write', [
                    'path' => $configPath,
                    'search' => '[certificate.default]',
                    'replace_block' => "[certificate.default]\ncert = \"{$expectedCert}\"\nprivate-key = \"%{file:/etc/letsencrypt/live/{$certDomain}/privkey.pem}%\"\ndefault = true",
                ]);
                $agent->send('service.restart', ['service' => 'stalwart-mail']);
                Log::info("Migration: Stalwart TLS updated via agent for {$certDomain}");
            } catch (\Exception $e) {
                Log::warning("Migration: Could not fix Stalwart TLS: {$e->getMessage()}");
            }

            return;
        }

        $expectedKey = "%{file:/etc/letsencrypt/live/{$certDomain}/privkey.pem}%";
        $config = preg_replace(
            '/\[certificate\.default\]\s*\n(?:(?:cert|private-key|default)\s*=\s*[^\n]+\n?)+/',
            "[certificate.default]\ncert = \"{$expectedCert}\"\nprivate-key = \"{$expectedKey}\"\ndefault = true\n",
            $config
        );

        File::put($configPath, $config);

        try {
            $agent = app(AgentClient::class);
            $agent->send('service.restart', ['service' => 'stalwart-mail']);
        } catch (\Exception $e) {
            Log::warning("Migration: Could not restart Stalwart: {$e->getMessage()}");
        }

        Log::info("Migration: Stalwart TLS updated for {$certDomain}");
    }

    private function ensureWebmasterMailbox(string $hostname): ?string
    {
        $existingMailbox = Mailbox::whereHas('emailDomain.domain', function ($q) use ($hostname) {
            $q->where('domain', $hostname);
        })->where('local_part', 'webmaster')->first();

        if ($existingMailbox) {
            return $existingMailbox->plain_password;
        }

        try {
            $agent = app(AgentClient::class);
            $admin = User::where('username', 'admin')->first();
            if (! $admin) {
                return null;
            }

            $domain = Domain::where('domain', $hostname)->first();
            if (! $domain) {
                try {
                    $agent->domainCreate($admin->username, $hostname);
                } catch (\Exception $e) {
                    // May already exist on disk
                }

                $domain = Domain::create([
                    'user_id' => $admin->id,
                    'domain' => $hostname,
                    'document_root' => "/home/{$admin->username}/domains/{$hostname}/public_html",
                    'is_active' => true,
                ]);
            }

            try {
                $agent->emailEnableDomain($admin->username, $hostname);
            } catch (\Exception $e) {
                // May already be enabled
            }

            $emailDomain = EmailDomain::firstOrCreate(
                ['domain_id' => $domain->id],
                ['is_active' => true]
            );

            $password = 'Jb!'.Str::random(14);
            $result = $agent->mailboxCreate($admin->username, "webmaster@{$hostname}", $password, 1073741824);

            Mailbox::create([
                'email_domain_id' => $emailDomain->id,
                'user_id' => $admin->id,
                'local_part' => 'webmaster',
                'password_hash' => $result['password_hash'] ?? '',
                'password_encrypted' => Crypt::encryptString($password),
                'maildir_path' => $result['maildir_path'] ?? null,
                'system_uid' => $result['uid'] ?? null,
                'system_gid' => $result['gid'] ?? null,
                'name' => 'System Webmaster',
                'quota_bytes' => 1073741824,
                'is_active' => true,
            ]);

            app(MailRoutingSyncService::class)->sync();
            Log::info("Migration: Webmaster mailbox created: webmaster@{$hostname}");

            return $password;
        } catch (\Exception $e) {
            Log::warning("Migration: Failed to create webmaster mailbox: {$e->getMessage()}");

            return null;
        }
    }

    private function configureSmtp(string $hostname, ?string $password): void
    {
        $envFile = base_path('.env');
        if (! File::exists($envFile)) {
            return;
        }

        $env = File::get($envFile);

        if (preg_match('/^MAIL_MAILER=smtp/m', $env) && preg_match('/^MAIL_USERNAME=webmaster@/m', $env)) {
            return;
        }

        $parts = explode('.', $hostname);
        $rootDomain = count($parts) > 2 ? implode('.', array_slice($parts, -2)) : $hostname;
        $mailHost = str_starts_with($hostname, 'mail.') ? $hostname : "mail.{$rootDomain}";

        if (! $password) {
            $mailbox = Mailbox::whereHas('emailDomain.domain', function ($q) use ($hostname) {
                $q->where('domain', $hostname);
            })->where('local_part', 'webmaster')->first();

            $password = $mailbox?->plain_password;
        }

        if (! $password) {
            return;
        }

        $replacements = [
            '/^MAIL_MAILER=.*/m' => 'MAIL_MAILER=smtp',
            '/^MAIL_HOST=.*/m' => "MAIL_HOST={$mailHost}",
            '/^MAIL_PORT=.*/m' => 'MAIL_PORT=587',
            '/^MAIL_ENCRYPTION=.*/m' => 'MAIL_ENCRYPTION=tls',
            '/^MAIL_USERNAME=.*/m' => "MAIL_USERNAME=webmaster@{$hostname}",
            '/^MAIL_PASSWORD=.*/m' => "MAIL_PASSWORD=\"{$password}\"",
        ];

        foreach ($replacements as $pattern => $replacement) {
            if (preg_match($pattern, $env)) {
                $env = preg_replace($pattern, $replacement, $env);
            }
        }

        $missing = [
            'MAIL_HOST' => $mailHost,
            'MAIL_PORT' => '587',
            'MAIL_ENCRYPTION' => 'tls',
            'MAIL_USERNAME' => "webmaster@{$hostname}",
            'MAIL_PASSWORD' => "\"{$password}\"",
        ];

        foreach ($missing as $key => $value) {
            if (! str_contains($env, "{$key}=")) {
                $env .= "\n{$key}={$value}";
            }
        }

        File::put($envFile, $env);
        Log::info("Migration: SMTP configured via webmaster@{$hostname}");
    }

    private function fixNginxConfigs(): void
    {
        if (! File::isDirectory('/etc/nginx/sites-enabled')) {
            return;
        }

        $brandingBlock = "    location /branding/ {\n        alias /opt/bulwark/public/branding/;\n        expires 7d;\n        access_log off;\n    }\n";
        $modified = false;

        foreach (array_merge(
            File::glob('/etc/nginx/sites-enabled/*'),
            File::glob('/etc/nginx/sites-available/*')
        ) as $conf) {
            if (! is_writable($conf)) {
                continue;
            }

            $content = File::get($conf);
            $changed = false;

            // Remove |storage from deny block
            if (str_contains($content, 'vendor|node_modules|storage')) {
                $content = str_replace('vendor|node_modules|storage', 'vendor|node_modules', $content);
                $changed = true;
            }

            // Add /branding/ alias if missing
            if (! str_contains($content, 'location /branding/') && str_contains($content, 'location = /webmail')) {
                $content = str_replace(
                    '    location = /webmail {',
                    "{$brandingBlock}\n    location = /webmail {",
                    $content
                );
                $changed = true;
            }

            if ($changed) {
                File::put($conf, $content);
                $modified = true;
            }
        }

        if ($modified) {
            try {
                $agent = app(AgentClient::class);
                $agent->send('service.reload', ['service' => 'nginx']);
                Log::info('Migration: Fixed nginx configs (storage access + branding alias)');
            } catch (\Exception $e) {
                Log::warning("Migration: Could not reload nginx: {$e->getMessage()}");
            }
        }
    }

    public function down(): void
    {
        // System config changes are not safely reversible
    }
};
