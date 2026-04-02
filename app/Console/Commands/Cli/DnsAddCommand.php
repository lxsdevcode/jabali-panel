<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use App\Models\DnsRecord;
use App\Models\Domain;
use App\Services\Dns\PowerDnsService;
use Illuminate\Console\Command;

class DnsAddCommand extends JabaliCommand
{
    protected $signature = 'jabali:dns:add {domain} {name} {type} {content} {--ttl=3600} {--priority=0} {--json} {--yes}';

    protected $description = 'Add a DNS record';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $domainName = $this->argument('domain');
        $domain = Domain::where('domain', $domainName)->first();

        if (! $domain) {
            $this->formatter()->error("Domain not found: {$domainName}");

            return Command::FAILURE;
        }

        $name = $this->argument('name');
        $type = strtoupper($this->argument('type'));
        $content = $this->argument('content');
        $ttl = (int) $this->option('ttl');
        $priority = (int) $this->option('priority');

        $validTypes = DnsRecord::getTypes();
        if (! in_array($type, $validTypes)) {
            $this->formatter()->error("Invalid DNS type: {$type}. Valid types: ".implode(', ', $validTypes));

            return Command::FAILURE;
        }

        try {
            $powerDns = app(PowerDnsService::class);
            $powerDns->addRecord($domainName, $name, $type, $content, $ttl, $priority);

            $record = DnsRecord::create([
                'domain_id' => $domain->id,
                'name' => $name,
                'type' => $type,
                'content' => $content,
                'ttl' => $ttl,
                'priority' => $priority,
            ]);

            if ($this->option('json')) {
                $this->formatter()->json([
                    'id' => $record->id,
                    'domain' => $domain->domain,
                    'name' => $record->name,
                    'type' => $record->type,
                    'content' => $record->content,
                    'ttl' => $record->ttl,
                    'priority' => $record->priority,
                    'full_name' => $record->full_name,
                ]);
            } else {
                $this->formatter()->success("DNS record created: {$record->full_name} {$type}");
            }

            return Command::SUCCESS;
        } catch (\Exception $e) {
            $this->formatter()->error("Failed to add DNS record: {$e->getMessage()}");

            return Command::FAILURE;
        }
    }
}
