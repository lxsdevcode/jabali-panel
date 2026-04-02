<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use App\Models\DnsRecord;
use App\Models\Domain;
use App\Services\Dns\PowerDnsService;
use Illuminate\Console\Command;

class DnsSyncCommand extends JabaliCommand
{
    protected $signature = 'jabali:dns:sync {domain} {--json} {--yes}';

    protected $description = 'Sync DNS zone with PowerDNS';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $domainName = $this->argument('domain');
        $domain = Domain::where('domain', $domainName)->first();

        if (! $domain) {
            $this->formatter()->error("Domain not found: {$domainName}");

            return Command::FAILURE;
        }

        try {
            $powerDns = app(PowerDnsService::class);

            // Fetch all records from database
            $records = DnsRecord::where('domain_id', $domain->id)->get();
            $recordsArray = $records->map(fn (DnsRecord $r) => [
                'name' => $r->name,
                'type' => $r->type,
                'content' => $r->content,
                'ttl' => $r->ttl,
                'priority' => $r->priority,
            ])->toArray();

            // Sync to PowerDNS
            $powerDns->setRecords($domainName, $recordsArray);

            if ($this->option('json')) {
                $this->formatter()->json([
                    'synced' => true,
                    'domain' => $domain->domain,
                    'records_count' => count($recordsArray),
                ]);
            } else {
                $this->formatter()->success("DNS zone synced: {$domain->domain} ({$records->count()} records)");
            }

            return Command::SUCCESS;
        } catch (\Exception $e) {
            $this->formatter()->error("Failed to sync DNS zone: {$e->getMessage()}");

            return Command::FAILURE;
        }
    }
}
