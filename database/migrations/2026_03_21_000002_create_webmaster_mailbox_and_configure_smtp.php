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

return new class extends Migration
{
    public function up(): void
    {
        if (config('jabali.mail_backend') !== 'stalwart') {
            return;
        }

        $hostname = DnsSetting::get('hostname', gethostname() ?: 'localhost');
        $password = $this->ensureWebmasterMailbox($hostname);
        $this->configureSmtp($hostname, $password);
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
                Log::warning('Migration: No admin user found, skipping webmaster mailbox');

                return null;
            }

            $domain = Domain::where('domain', $hostname)->first();
            if (! $domain) {
                try {
                    $agent->domainCreate($admin->username, $hostname);
                } catch (Exception $e) {
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
            } catch (Exception $e) {
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
        } catch (Exception $e) {
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

        // Skip if already using authenticated SMTP
        if (preg_match('/^MAIL_MAILER=smtp/m', $env) && preg_match('/^MAIL_USERNAME=webmaster@/m', $env)) {
            return;
        }

        $mailHost = str_starts_with($hostname, 'mail.') ? $hostname : "mail.{$hostname}";

        if (! $password) {
            $mailbox = Mailbox::whereHas('emailDomain.domain', function ($q) use ($hostname) {
                $q->where('domain', $hostname);
            })->where('local_part', 'webmaster')->first();

            $password = $mailbox?->plain_password;
        }

        if (! $password) {
            Log::warning('Migration: No webmaster password available, skipping SMTP config');

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

    public function down(): void
    {
        // Reverting mail config changes is not safe automatically
    }
};
