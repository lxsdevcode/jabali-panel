<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use App\Models\Domain;
use App\Models\User;
use Illuminate\Console\Command;

class UserShowCommand extends JabaliCommand
{
    protected $signature = 'jabali:user:show {username} {--json} {--yes}';

    protected $description = 'Show details of a user';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $username = $this->argument('username');
        $user = User::where('username', $username)->orWhere('name', $username)->first();

        if (! $user) {
            $this->formatter()->error("User not found: {$username}");

            return Command::FAILURE;
        }

        $domainCount = Domain::where('user_id', $user->id)->count();

        if ($this->option('json')) {
            $this->formatter()->json([
                'id' => $user->id,
                'username' => $user->username ?? $user->name,
                'email' => $user->email,
                'role' => $user->is_admin ? 'Admin' : 'User',
                'status' => $user->is_active ? 'Active' : 'Suspended',
                'created_at' => $user->created_at?->toDateTimeString(),
                'domain_count' => $domainCount,
            ]);

            return Command::SUCCESS;
        }

        $status = ! $user->is_active
            ? '<fg=red>Suspended</>'
            : '<fg=green>Active</>';

        $this->formatter()->table(
            ['Field', 'Value'],
            [
                ['ID', (string) $user->id],
                ['Username', $user->username ?? $user->name],
                ['Email', $user->email],
                ['Role', $user->is_admin ? 'Admin' : 'User'],
                ['Status', $status],
                ['Created', $user->created_at?->toDateTimeString() ?? '-'],
                ['Domains', (string) $domainCount],
            ],
        );

        return Command::SUCCESS;
    }
}
