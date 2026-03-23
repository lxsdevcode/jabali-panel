<?php

declare(strict_types=1);

namespace App\Jobs;

use App\Models\Backup;
use App\Models\BackupSchedule;
use App\Services\AdminNotificationService;
use App\Services\Agent\AgentClient;
use App\Services\Backup\BackupOrchestrator;
use Exception;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Queue\Queueable;
use Illuminate\Support\Facades\Log;

class RunServerBackup implements ShouldQueue
{
    use Queueable;

    public int $tries = 1;

    public int $timeout = 3600; // 1 hour max

    public function __construct(
        public int $backupId
    ) {}

    public function handle(): void
    {
        $backup = Backup::find($this->backupId);

        if (! $backup) {
            Log::warning("RunServerBackup: Backup {$this->backupId} not found");

            return;
        }

        // Skip if already completed or failed
        if (in_array($backup->status, ['completed', 'failed'])) {
            Log::info("RunServerBackup: Backup {$this->backupId} already {$backup->status}");

            return;
        }

        $backup->update(['status' => 'running', 'started_at' => now()]);

        $backupType = $backup->metadata['backup_type'] ?? 'full';
        $isIncrementalRemote = $backupType === 'incremental' && $backup->destination_id;
        $orchestrator = app(BackupOrchestrator::class);

        try {
            $agent = app(AgentClient::class);

            if ($isIncrementalRemote) {
                $destination = $backup->destination;
                if (! $destination) {
                    throw new Exception('Backup destination not found');
                }

                $config = $orchestrator->buildDestinationConfig($destination);

                $result = $agent->send('backup.incremental_direct', [
                    'destination' => $config,
                    'users' => $backup->users,
                    'include_files' => $backup->include_files,
                    'include_databases' => $backup->include_databases,
                    'include_mailboxes' => $backup->include_mailboxes,
                    'include_dns' => $backup->include_dns,
                ]);

                if ($result['success'] ?? false) {
                    $backup->update([
                        'status' => 'completed',
                        'completed_at' => now(),
                        'size_bytes' => $result['size'] ?? 0,
                        'users' => $result['users'] ?? $backup->users,
                        'remote_path' => $result['remote_path'] ?? null,
                        'metadata' => array_merge($backup->metadata ?? [], [
                            'user_count' => $result['user_count'] ?? 0,
                            'previous_backup' => $result['previous_backup'] ?? null,
                            'is_initialization' => $result['is_initialization'] ?? false,
                        ]),
                    ]);

                    Log::info("RunServerBackup: Backup {$this->backupId} completed successfully (incremental)");

                    // Re-index remote backups for user discovery
                    IndexRemoteBackups::dispatch($backup->destination_id);

                    // Apply retention policy if this backup is from a schedule
                    if ($backup->schedule_id) {
                        $schedule = BackupSchedule::find($backup->schedule_id);
                        if ($schedule) {
                            $orchestrator->applyRetention($schedule);
                        }
                    }

                    // Send success notification
                    AdminNotificationService::backupSuccess(
                        $backup->name,
                        $result['size'] ?? 0,
                        $backup->destination?->name
                    );
                } else {
                    throw new Exception($result['error'] ?? 'Incremental backup failed');
                }
            } else {
                // Full backup
                $outputPath = $backup->local_path;

                $result = $agent->send('backup.create_server', [
                    'output_path' => $outputPath,
                    'backup_type' => $backupType,
                    'users' => $backup->users,
                    'include_files' => $backup->include_files,
                    'include_databases' => $backup->include_databases,
                    'include_mailboxes' => $backup->include_mailboxes,
                    'include_dns' => $backup->include_dns,
                ]);

                if ($result['success'] ?? false) {
                    $backup->update([
                        'status' => 'completed',
                        'completed_at' => now(),
                        'size_bytes' => $result['size'] ?? 0,
                        'users' => $result['users'] ?? $backup->users,
                        'metadata' => array_merge($backup->metadata ?? [], [
                            'user_count' => $result['user_count'] ?? 0,
                        ]),
                    ]);

                    // Upload to remote if destination configured
                    if ($backup->destination_id) {
                        $orchestrator->uploadToRemote($backup);
                    }

                    Log::info("RunServerBackup: Backup {$this->backupId} completed successfully (full)");

                    // Apply retention policy if this backup is from a schedule
                    if ($backup->schedule_id) {
                        $schedule = BackupSchedule::find($backup->schedule_id);
                        if ($schedule) {
                            $orchestrator->applyRetention($schedule);
                        }
                    }

                    // Send success notification
                    AdminNotificationService::backupSuccess(
                        $backup->name,
                        $result['size'] ?? 0,
                        $backup->destination?->name
                    );
                } else {
                    throw new Exception($result['error'] ?? 'Backup failed');
                }
            }
        } catch (Exception $e) {
            $backup->update([
                'status' => 'failed',
                'completed_at' => now(),
                'error_message' => $e->getMessage(),
            ]);

            Log::error("RunServerBackup: Backup {$this->backupId} failed: ".$e->getMessage());

            // Send failure notification
            AdminNotificationService::backupFailure($backup->name, $e->getMessage());
        }
    }
}
