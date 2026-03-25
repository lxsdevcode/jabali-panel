<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use App\Models\Domain;
use Illuminate\Console\Command;

class DomainShowCommand extends JabaliCommand
{
    protected $signature = 'jabali:domain:show {domain} {--json} {--yes}';

    protected $description = 'Show details of a domain';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $domainName = $this->argument('domain');
        $domain = Domain::with('user')->where('domain', $domainName)->first();

        if (! $domain) {
            $this->formatter()->error("Domain not found: {$domainName}");

            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json([
                'id' => $domain->id,
                'domain' => $domain->domain,
                'user' => $domain->user?->username,
                'is_active' => $domain->is_active,
                'ssl_enabled' => $domain->ssl_enabled,
                'created_at' => $domain->created_at?->toDateTimeString(),
            ]);

            return Command::SUCCESS;
        }

        $status = $domain->is_active ? '<fg=green>Active</>' : '<fg=red>Inactive</>';
        $ssl = $domain->ssl_enabled ? '<fg=green>SSL</>' : '<fg=gray>No SSL</>';

        $this->formatter()->table(
            ['Field', 'Value'],
            [
                ['ID', (string) $domain->id],
                ['Domain', $domain->domain],
                ['User', $domain->user?->username ?? '-'],
                ['Status', $status],
                ['SSL', $ssl],
                ['Created', $domain->created_at?->toDateTimeString() ?? '-'],
            ],
        );

        return Command::SUCCESS;
    }
}
