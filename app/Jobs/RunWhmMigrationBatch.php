<?php

declare(strict_types=1);

namespace App\Jobs;

use App\Services\Migration\WhmMigrationOrchestrator;
use App\Services\Migration\WhmMigrationStatusStore;
use Illuminate\Contracts\Queue\ShouldBeEncrypted;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Queue\Queueable;
use Illuminate\Support\Facades\Log;
use Throwable;

class RunWhmMigrationBatch implements ShouldBeEncrypted, ShouldQueue
{
    use Queueable;

    public int $tries = 1;

    public int $timeout = 7200;

    /**
     * @param  array<int, array<string, mixed>>  $accounts
     * @param  array<int, string>  $selectedAccounts
     */
    public function __construct(
        public string $cacheKey,
        public string $hostname,
        public string $whmUsername,
        public string $apiToken,
        public int $port,
        public bool $useSSL,
        public array $accounts,
        public array $selectedAccounts,
        public bool $restoreFiles,
        public bool $restoreDatabases,
        public bool $restoreEmails,
        public bool $restoreSsl,
        public bool $createLinuxUsers,
    ) {}

    public function handle(WhmMigrationOrchestrator $orchestrator): void
    {
        $orchestrator->run(
            $this->cacheKey,
            $this->hostname,
            $this->whmUsername,
            $this->apiToken,
            $this->port,
            $this->useSSL,
            $this->accounts,
            $this->selectedAccounts,
            $this->restoreFiles,
            $this->restoreDatabases,
            $this->restoreEmails,
            $this->restoreSsl,
            $this->createLinuxUsers,
        );
    }

    public function failed(Throwable $exception): void
    {
        $store = new WhmMigrationStatusStore($this->cacheKey);
        $store->setMigrating(false);

        Log::error('RunWhmMigrationBatch job failed', [
            'cache_key' => $this->cacheKey,
            'hostname' => $this->hostname,
            'error' => $exception->getMessage(),
        ]);
    }
}
