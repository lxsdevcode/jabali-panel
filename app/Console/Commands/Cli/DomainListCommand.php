<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use App\Models\Domain;
use App\Models\User;
use Illuminate\Console\Command;

class DomainListCommand extends JabaliCommand
{
    protected $signature = 'jabali:domain:list {--user=} {--json} {--yes}';

    protected $description = 'List all domains';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $query = Domain::with('user');

        if ($username = $this->option('user')) {
            $user = User::where('username', $username)->first();

            if (! $user) {
                $this->formatter()->error("User not found: {$username}");

                return Command::FAILURE;
            }

            $query->where('user_id', $user->id);
        }

        $domains = $query->get();

        if ($this->option('json')) {
            $this->formatter()->json($domains->map(fn (Domain $d) => [
                'id' => $d->id,
                'domain' => $d->domain,
                'user' => $d->user?->username,
                'is_active' => $d->is_active,
                'ssl_enabled' => $d->ssl_enabled,
                'created_at' => $d->created_at?->toDateTimeString(),
            ])->toArray());

            return Command::SUCCESS;
        }

        $rows = [];
        foreach ($domains as $domain) {
            $status = $domain->is_active ? '<fg=green>Active</>' : '<fg=red>Inactive</>';
            $ssl = $domain->ssl_enabled ? '<fg=green>SSL</>' : '<fg=gray>No SSL</>';
            $rows[] = [
                $domain->id,
                $domain->domain,
                $domain->user?->username ?? '-',
                $status,
                $ssl,
                $domain->created_at?->toDateTimeString() ?? '-',
            ];
        }

        $this->formatter()->table(['ID', 'Domain', 'User', 'Status', 'SSL', 'Created'], $rows);

        return Command::SUCCESS;
    }
}
