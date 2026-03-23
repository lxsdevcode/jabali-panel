<?php

declare(strict_types=1);

namespace App\Console\Commands;

use App\Models\Backup;
use App\Models\BackupSchedule;
use App\Services\AdminNotificationService;
use App\Services\Agent\AgentClient;
use App\Services\Backup\BackupOrchestrator;
use Exception;
use Illuminate\Console\Command;

class RunBackupSchedules extends Command
{
    protected $signature = 'backups:run-schedules';

    protected $description = 'Run due backup schedules';

    protected AgentClient $agent;

    protected BackupOrchestrator $orchestrator;

    public function __construct()
    {
        parent::__construct();
        $this->agent = app(AgentClient::class);
        $this->orchestrator = app(BackupOrchestrator::class);
    }

    public function handle(): int
    {
        $this->info('Checking for due backup schedules...');

        $dueSchedules = BackupSchedule::due()->with('destination')->get();

        if ($dueSchedules->isEmpty()) {
            $this->info('No backup schedules due.');

            return Command::SUCCESS;
        }

        $this->info("Found {$dueSchedules->count()} schedule(s) to run.");

        foreach ($dueSchedules as $schedule) {
            $this->runSchedule($schedule);
        }

        return Command::SUCCESS;
    }

    protected function runSchedule(BackupSchedule $schedule): void
    {
        $this->info("Running schedule: {$schedule->name}");

        $backupType = $schedule->metadata['backup_type'] ?? 'full';
        $timestamp = now()->format('Y-m-d_His');

        if ($schedule->is_server_backup) {
            // Server backup: folder with individual user tar.gz files
            $filename = $timestamp;
            $outputPath = "/var/backups/jabali/{$timestamp}";
        } else {
            $user = $schedule->user;
            if (! $user) {
                $this->error("Schedule {$schedule->id} has no user.");

                return;
            }
            $filename = "backup_scheduled_{$timestamp}.tar.gz";
            $outputPath = "/home/{$user->username}/backups/{$filename}";
        }

        $backup = Backup::create([
            'user_id' => $schedule->user_id,
            'destination_id' => $schedule->destination_id,
            'schedule_id' => $schedule->id,
            'name' => "{$schedule->name} - ".now()->format('M j, Y H:i'),
            'filename' => $filename,
            'type' => $schedule->is_server_backup ? 'server' : 'partial',
            'include_files' => $schedule->include_files,
            'include_databases' => $schedule->include_databases,
            'include_mailboxes' => $schedule->include_mailboxes,
            'include_dns' => $schedule->include_dns,
            'users' => $schedule->users,
            'status' => 'pending',
            'local_path' => $outputPath,
            'metadata' => ['backup_type' => $backupType],
        ]);

        try {
            $backup->update(['status' => 'running', 'started_at' => now()]);

            // Check if this is an incremental backup with remote destination (dirvish-style)
            $isIncrementalRemote = $backupType === 'incremental' && $schedule->destination;

            if ($schedule->is_server_backup) {
                if ($isIncrementalRemote) {
                    // Dirvish-style: rsync directly to remote with --link-dest
                    $config = $this->orchestrator->buildDestinationConfig($schedule->destination);
                    $result = $this->agent->backupIncrementalDirect($config, [
                        'users' => $schedule->users,
                        'include_files' => $schedule->include_files,
                        'include_databases' => $schedule->include_databases,
                        'include_mailboxes' => $schedule->include_mailboxes,
                        'include_dns' => $schedule->include_dns,
                    ]);

                    // Update backup record with remote path
                    if ($result['success']) {
                        $backup->update([
                            'local_path' => null, // No local file for incremental remote
                            'remote_path' => $result['remote_path'] ?? null,
                        ]);
                    }
                } else {
                    $result = $this->agent->backupCreateServer($outputPath, [
                        'backup_type' => $backupType,
                        'users' => $schedule->users,
                        'include_files' => $schedule->include_files,
                        'include_databases' => $schedule->include_databases,
                        'include_mailboxes' => $schedule->include_mailboxes,
                        'include_dns' => $schedule->include_dns,
                    ]);
                }
            } else {
                $result = $this->agent->backupCreate($schedule->user->username, $outputPath, [
                    'include_files' => $schedule->include_files,
                    'include_databases' => $schedule->include_databases,
                    'include_mailboxes' => $schedule->include_mailboxes,
                    'include_dns' => $schedule->include_dns,
                    'include_ssl' => $schedule->include_ssl ?? true,
                ]);
            }

            if ($result['success']) {
                $backup->update([
                    'status' => 'completed',
                    'completed_at' => now(),
                    'size_bytes' => $result['size'] ?? 0,
                    'checksum' => $result['checksum'] ?? null,
                    'domains' => $result['domains'] ?? null,
                    'databases' => $result['databases'] ?? null,
                    'mailboxes' => $result['mailboxes'] ?? null,
                    'users' => $result['users'] ?? null,
                ]);

                // Upload to remote if destination configured
                if ($schedule->destination) {
                    $backup->load('destination'); // Load the relationship
                    $keepLocal = $schedule->metadata['keep_local_copy'] ?? false;
                    $uploadSuccess = $this->orchestrator->uploadToRemote($backup, $keepLocal);

                    if ($uploadSuccess) {
                        $this->info("Uploaded to remote: {$backup->destination->name}");
                    }
                }

                $schedule->update([
                    'last_run_at' => now(),
                    'last_status' => 'success',
                    'last_error' => null,
                ]);

                $this->info("Backup completed: {$backup->name}");

                // Apply retention policy
                $this->orchestrator->applyRetention($schedule);
            } else {
                throw new Exception($result['error'] ?? 'Backup failed');
            }
        } catch (Exception $e) {
            $backup->update([
                'status' => 'failed',
                'completed_at' => now(),
                'error_message' => $e->getMessage(),
            ]);

            $schedule->update([
                'last_run_at' => now(),
                'last_status' => 'failed',
                'last_error' => $e->getMessage(),
            ]);

            $this->error("Backup failed: {$e->getMessage()}");

            // Send admin notification
            AdminNotificationService::backupFailure($schedule->name, $e->getMessage());
        }

        // Calculate next run
        $schedule->calculateNextRun();
        $schedule->save();
    }
}
