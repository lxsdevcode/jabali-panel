<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use App\Models\DnsRecord;
use App\Models\Domain;
use Illuminate\Console\Command;

class DnsRecordsCommand extends JabaliCommand
{
    protected $signature = 'jabali:dns:records {domain} {--type=} {--json} {--yes}';

    protected $description = 'List DNS records for a domain';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $domainName = $this->argument('domain');
        $domain = Domain::where('domain', $domainName)->first();

        if (! $domain) {
            $this->formatter()->error("Domain not found: {$domainName}");

            return Command::FAILURE;
        }

        $query = DnsRecord::where('domain_id', $domain->id);

        if ($type = $this->option('type')) {
            $query->where('type', strtoupper($type));
        }

        $records = $query->orderBy('type')->orderBy('name')->get();

        if ($this->option('json')) {
            $this->formatter()->json($records->map(fn (DnsRecord $r) => [
                'id' => $r->id,
                'name' => $r->name,
                'type' => $r->type,
                'content' => $r->content,
                'ttl' => $r->ttl,
                'priority' => $r->priority,
                'full_name' => $r->full_name,
            ])->toArray());

            return Command::SUCCESS;
        }

        $rows = [];
        foreach ($records as $record) {
            $priority = $record->priority > 0 ? (string) $record->priority : '-';
            $rows[] = [
                $record->id,
                $record->name,
                $record->type,
                $record->content,
                $record->ttl,
                $priority,
            ];
        }

        $this->formatter()->table(['ID', 'Name', 'Type', 'Content', 'TTL', 'Priority'], $rows);

        return Command::SUCCESS;
    }
}
