<?php

declare(strict_types=1);

namespace App\Jobs;

use App\Models\Backup;
use App\Services\Backup\BackupOrchestrator;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Queue\Queueable;
use Illuminate\Support\Facades\Log;

class RunServerBackup implements ShouldQueue
{
    use Queueable;

    public int $tries = 1;

    public int $timeout = 3600;

    public function __construct(
        public int $backupId
    ) {}

    public function handle(BackupOrchestrator $orchestrator): void
    {
        $backup = Backup::find($this->backupId);

        if (! $backup) {
            Log::warning("RunServerBackup: Backup {$this->backupId} not found");

            return;
        }

        $orchestrator->execute($backup);
    }
}
