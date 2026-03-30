<?php

declare(strict_types=1);

namespace App\Console\Commands;

use App\Models\User;
use Illuminate\Console\Command;
use Illuminate\Support\Facades\Cache;
use Illuminate\Support\Str;

class GenerateLoginTokenCommand extends Command
{
    protected $signature = 'jabali:login-token
        {--user=admin : Username to generate token for}
        {--ttl=5 : Token lifetime in minutes}
        {--panel=admin : Panel to redirect to (admin or user)}';

    protected $description = 'Generate a one-time login URL for automated testing or emergency access';

    public function handle(): int
    {
        $username = $this->option('user');
        $ttl = max(1, min(60, (int) $this->option('ttl')));
        $panel = $this->option('panel');

        $user = User::where('username', $username)->first();
        if (! $user) {
            $this->error("User '{$username}' not found.");

            return self::FAILURE;
        }

        $token = Str::random(64);
        $cacheKey = 'login_token:'.hash('sha256', $token);

        Cache::put($cacheKey, [
            'user_id' => $user->id,
            'panel' => $panel,
        ], now()->addMinutes($ttl));

        $hostname = config('jabali.panel.hostname') ?: config('app.url');
        $url = rtrim($hostname, '/');
        if (! str_starts_with($url, 'http')) {
            $url = "https://{$url}:2223";
        }
        $url .= "/auto-login?token={$token}";

        $this->newLine();
        $this->info("Login URL for '{$username}' (expires in {$ttl} min):");
        $this->newLine();
        $this->line("  {$url}");
        $this->newLine();
        $this->warn('This is a one-time use token. It will be invalidated after first use.');

        return self::SUCCESS;
    }
}
