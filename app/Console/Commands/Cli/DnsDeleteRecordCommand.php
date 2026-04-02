<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use App\Models\DnsRecord;
use App\Models\Domain;
use App\Services\Dns\PowerDnsService;
use Illuminate\Console\Command;

class DnsDeleteRecordCommand extends JabaliCommand
{
    protected $signature = 'jabali:dns:delete-record {domain} {name} {type} {--json} {--yes}';

    protected $description = 'Delete a DNS record';

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

        $record = DnsRecord::where('domain_id', $domain->id)
            ->where('name', $name)
            ->where('type', $type)
            ->first();

        if (! $record) {
            $this->formatter()->error("DNS record not found: {$name} {$type}");

            return Command::FAILURE;
        }

        $message = "Delete DNS record: {$record->full_name} {$type}?";
        if (! $this->confirmAction($message)) {
            $this->formatter()->success('Cancelled');

            return Command::SUCCESS;
        }

        try {
            $powerDns = app(PowerDnsService::class);
            $powerDns->deleteRecord($domainName, $name, $type);

            $record->delete();

            if ($this->option('json')) {
                $this->formatter()->json([
                    'deleted' => true,
                    'domain' => $domain->domain,
                    'name' => $name,
                    'type' => $type,
                ]);
            } else {
                $this->formatter()->success("DNS record deleted: {$record->full_name} {$type}");
            }

            return Command::SUCCESS;
        } catch (\Exception $e) {
            $this->formatter()->error("Failed to delete DNS record: {$e->getMessage()}");

            return Command::FAILURE;
        }
    }
}
