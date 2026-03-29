<?php

declare(strict_types=1);

namespace App\Console\Commands;

use App\Models\Domain;
use App\Models\DomainBandwidthUsage;
use App\Services\Agent\AgentClient;
use Illuminate\Console\Command;

class SyncBandwidthCommand extends Command
{
    protected $signature = 'jabali:sync-bandwidth';

    protected $description = 'Sync bandwidth usage from nginx access logs for all domains';

    public function handle(): int
    {
        $agent = app(AgentClient::class);
        $domains = Domain::with('user')->where('is_active', true)->get();
        $date = now()->toDateString();
        $synced = 0;

        foreach ($domains as $domain) {
            if (! $domain->user) {
                continue;
            }

            try {
                $result = $agent->send('usage.domain_bandwidth', [
                    'username' => $domain->user->username,
                    'domain' => $domain->domain,
                ]);

                if ($result['success'] ?? false) {
                    DomainBandwidthUsage::updateOrCreate(
                        ['domain_id' => $domain->id, 'date' => $date],
                        [
                            'bytes_out' => $result['bytes_out'] ?? 0,
                            'requests' => $result['requests'] ?? 0,
                        ]
                    );
                    $synced++;
                }
            } catch (\Exception $e) {
                $this->warn("Failed for {$domain->domain}: {$e->getMessage()}");
            }
        }

        $this->info("Synced bandwidth for {$synced}/{$domains->count()} domains.");

        return self::SUCCESS;
    }
}
