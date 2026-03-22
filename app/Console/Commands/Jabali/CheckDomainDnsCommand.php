<?php

declare(strict_types=1);

namespace App\Console\Commands\Jabali;

use App\Models\Domain;
use App\Services\DomainHealthService;
use Illuminate\Console\Command;

class CheckDomainDnsCommand extends Command
{
    protected $signature = 'jabali:check-domain-dns
                            {--domain= : Check a single domain}';

    protected $description = 'Check DNS resolution status for all domains';

    public function handle(DomainHealthService $service): int
    {
        $query = Domain::query();

        if ($domainName = $this->option('domain')) {
            $query->where('domain', $domainName);
        }

        $domains = $query->get();

        if ($domains->isEmpty()) {
            $this->warn('No domains found.');

            return self::SUCCESS;
        }

        $this->info("Checking DNS for {$domains->count()} domain(s)...");

        $checked = 0;
        foreach ($domains as $domain) {
            $result = $service->checkDns($domain);

            $domain->update([
                'dns_status' => $result['status'],
                'dns_resolved_ip' => $result['resolved_ip'],
                'dns_checked_at' => now(),
            ]);

            $this->line("  {$domain->domain}: {$result['status']}".($result['resolved_ip'] ? " ({$result['resolved_ip']})" : ''));
            $checked++;
        }

        $this->info("Done. Checked {$checked} domain(s).");

        return self::SUCCESS;
    }
}
