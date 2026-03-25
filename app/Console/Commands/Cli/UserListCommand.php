<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use App\Models\User;
use Illuminate\Console\Command;

class UserListCommand extends JabaliCommand
{
    protected $signature = 'jabali:user:list {--json} {--yes}';

    protected $description = 'List all users';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $users = User::all();

        if ($this->option('json')) {
            $this->formatter()->json($users->map(fn (User $u) => [
                'id' => $u->id,
                'username' => $u->username ?? $u->name,
                'email' => $u->email,
                'role' => $u->is_admin ? 'Admin' : 'User',
                'status' => $u->is_active ? 'Active' : 'Suspended',
                'created_at' => $u->created_at?->toDateTimeString(),
            ])->toArray());

            return Command::SUCCESS;
        }

        $rows = [];
        foreach ($users as $user) {
            $status = ! $user->is_active
                ? '<fg=red>Suspended</>'
                : '<fg=green>Active</>';
            $rows[] = [
                $user->id,
                $user->username ?? $user->name,
                $user->email,
                $user->is_admin ? 'Admin' : 'User',
                $status,
                $user->created_at?->format('Y-m-d') ?? '-',
            ];
        }

        $this->formatter()->table(['ID', 'Username', 'Email', 'Role', 'Status', 'Created'], $rows);

        return Command::SUCCESS;
    }
}
