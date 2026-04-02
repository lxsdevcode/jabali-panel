<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use App\Models\Domain;
use Illuminate\Console\Command;

class DnsListCommand extends JabaliCommand
{
    protected $signature = 'jabali:dns:list {--json} {--yes}';

    protected $description = 'List all DNS zones';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $domains = Domain::orderBy('domain')->get();

        if ($this->option('json')) {
            $this->formatter()->json($domains->map(fn (Domain $d) => [
                'id' => $d->id,
                'domain' => $d->domain,
                'is_active' => $d->is_active,
                'created_at' => $d->created_at?->toDateTimeString(),
            ])->toArray());

            return Command::SUCCESS;
        }

        $rows = [];
        foreach ($domains as $domain) {
            $status = $domain->is_active ? '<fg=green>Active</>' : '<fg=red>Inactive</>';
            $rows[] = [
                $domain->id,
                $domain->domain,
                $status,
                $domain->created_at?->toDateTimeString() ?? '-',
            ];
        }

        $this->formatter()->table(['ID', 'Domain', 'Status', 'Created'], $rows);

        return Command::SUCCESS;
    }
}
