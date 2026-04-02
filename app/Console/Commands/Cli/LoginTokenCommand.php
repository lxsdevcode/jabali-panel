<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use App\Models\AuditLog;
use App\Models\User;
use Illuminate\Console\Command;
use Illuminate\Support\Facades\Cache;
use Illuminate\Support\Str;

class LoginTokenCommand extends JabaliCommand
{
    protected $signature = 'jabali:login:token {--user=} {--ttl=15} {--panel=} {--json} {--yes}';

    protected $description = 'Generate a one-time login URL';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $username = $this->option('user');
        $ttl = max(1, min(60, (int) $this->option('ttl')));

        if (! $username) {
            $users = User::orderBy('username')->get(['id', 'username', 'is_admin']);

            if ($users->isEmpty()) {
                $this->formatter()->error('No users found');

                return Command::FAILURE;
            }

            $choices = $users->map(fn (User $u) => $u->username.($u->is_admin ? ' (admin)' : ''))->toArray();
            $selected = $this->choice('Select user', $choices);
            $username = explode(' ', $selected)[0];
        }

        $user = User::where('username', $username)->first();
        if (! $user) {
            $this->formatter()->error("User not found: {$username}");

            return Command::FAILURE;
        }

        $panel = $this->option('panel') ?: ($user->is_admin ? 'admin' : 'user');
        $token = Str::random(64);
        $cacheKey = 'login_token:'.hash('sha256', $token);

        Cache::put($cacheKey, [
            'user_id' => $user->id,
            'panel' => $panel,
        ], now()->addMinutes($ttl));

        $hostname = config('jabali.panel.hostname') ?: config('app.url');
        $url = rtrim((string) $hostname, '/');
        if (! str_starts_with($url, 'http')) {
            $url = "https://{$url}:".config('jabali.panel.port', 8443);
        }
        $url .= "/auto-login?token={$token}";

        AuditLog::log(
            'login_token_generated',
            'auth',
            "Login token generated for {$username} ({$panel} panel, {$ttl} min TTL)",
            'user',
            $user->id,
            $username,
            ['panel' => $panel, 'ttl' => $ttl]
        );

        if ($this->option('json')) {
            $this->formatter()->json([
                'url' => $url,
                'username' => $username,
                'panel' => $panel,
                'ttl_minutes' => $ttl,
            ]);

            return Command::SUCCESS;
        }

        $this->formatter()->success("Login URL for '{$username}' ({$panel} panel, expires in {$ttl} min):");
        $this->line('');
        $this->line("  {$url}");
        $this->line('');
        $this->formatter()->info('One-time use. Invalidated after first use or expiry.');

        return Command::SUCCESS;
    }
}
