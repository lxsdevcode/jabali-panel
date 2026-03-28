<?php

declare(strict_types=1);

namespace App\Services\Backup;

use App\Jobs\IndexRemoteBackups;
use App\Models\Backup;
use App\Models\BackupDestination;
use App\Models\BackupRestore;
use App\Models\BackupSchedule;
use App\Models\User;
use App\Services\AdminNotificationService;
use App\Services\Agent\AgentClient;
use Exception;
use Illuminate\Support\Facades\Log;

class BackupOrchestrator
{
    public function __construct(private AgentClient $agent) {}

    /**
     * Build the destination config array for agent calls.
     *
     * @return array<string, mixed>
     */
    public function execute(Backup $backup): void
    {
        if (in_array($backup->status, ['completed', 'failed'])) {
            return;
        }

        $backup->update(['status' => 'running', 'started_at' => now()]);

        $backupType = $backup->metadata['backup_type'] ?? 'full';
        $isIncrementalRemote = $backupType === 'incremental' && $backup->destination_id;

        try {
            $result = $isIncrementalRemote
                ? $this->executeIncremental($backup)
                : $this->executeFull($backup, $backupType);

            $this->handleSuccess($backup, $result, $isIncrementalRemote);
        } catch (Exception $e) {
            $backup->update([
                'status' => 'failed',
                'completed_at' => now(),
                'error_message' => $e->getMessage(),
            ]);

            Log::error("BackupOrchestrator: Backup {$backup->id} failed: ".$e->getMessage());
            AdminNotificationService::backupFailure($backup->name, $e->getMessage());
        }
    }

    /**
     * @return array<string, mixed>
     */
    private function executeIncremental(Backup $backup): array
    {
        $destination = $backup->destination;
        if (! $destination) {
            throw new Exception('Backup destination not found');
        }

        $config = $this->buildDestinationConfig($destination);

        $result = $this->agent->backupIncrementalDirect($config, [
            'users' => $backup->users,
            'include_files' => $backup->include_files,
            'include_databases' => $backup->include_databases,
            'include_mailboxes' => $backup->include_mailboxes,
            'include_dns' => $backup->include_dns,
            'include_ssl' => $backup->include_ssl ?? true,
        ]);

        if (! ($result['success'] ?? false)) {
            throw new Exception($result['error'] ?? 'Incremental backup failed');
        }

        return $result;
    }

    /**
     * @return array<string, mixed>
     */
    private function executeFull(Backup $backup, string $backupType): array
    {
        $result = $this->agent->backupCreateServer($backup->local_path, [
            'backup_type' => $backupType,
            'users' => $backup->users,
            'include_files' => $backup->include_files,
            'include_databases' => $backup->include_databases,
            'include_mailboxes' => $backup->include_mailboxes,
            'include_dns' => $backup->include_dns,
            'include_ssl' => $backup->include_ssl ?? true,
        ]);

        if (! ($result['success'] ?? false)) {
            throw new Exception($result['error'] ?? 'Backup failed');
        }

        return $result;
    }

    /**
     * @param  array<string, mixed>  $result
     */
    private function handleSuccess(Backup $backup, array $result, bool $isIncremental): void
    {
        $metadata = array_merge($backup->metadata ?? [], [
            'user_count' => $result['user_count'] ?? 0,
        ]);

        if ($isIncremental) {
            $metadata['previous_backup'] = $result['previous_backup'] ?? null;
            $metadata['is_initialization'] = $result['is_initialization'] ?? false;
        }

        $backup->update([
            'status' => 'completed',
            'completed_at' => now(),
            'size_bytes' => $result['size'] ?? 0,
            'users' => $result['users'] ?? $backup->users,
            'remote_path' => $result['remote_path'] ?? $backup->remote_path,
            'metadata' => $metadata,
        ]);

        Log::info("BackupOrchestrator: Backup {$backup->id} completed successfully (".($isIncremental ? 'incremental' : 'full').')');

        if ($isIncremental && $backup->destination_id) {
            IndexRemoteBackups::dispatch($backup->destination_id);
        } elseif ($backup->destination_id) {
            $this->uploadToRemote($backup);
        }

        if ($backup->schedule_id) {
            $schedule = BackupSchedule::find($backup->schedule_id);
            if ($schedule) {
                $this->applyRetention($schedule);
            }
        }

        AdminNotificationService::backupSuccess(
            $backup->name,
            $result['size'] ?? 0,
            $backup->destination?->name
        );
    }

    /**
     * Build the destination config array for agent calls.
     *
     * @return array<string, mixed>
     */
    public function buildDestinationConfig(BackupDestination $destination): array
    {
        return array_merge($destination->config ?? [], ['type' => $destination->type]);
    }

    /**
     * Test a backup destination connection.
     *
     * @return array<string, mixed>
     */
    public function testDestination(BackupDestination $destination): array
    {
        $config = $this->buildDestinationConfig($destination);
        $result = $this->agent->backupTestDestination($config);

        $destination->update([
            'last_tested_at' => now(),
            'test_status' => ($result['success'] ?? false) ? 'success' : 'failed',
            'test_message' => $result['message'] ?? $result['error'] ?? null,
        ]);

        if ($result['success'] ?? false) {
            Log::info("BackupDestination: Test passed for '{$destination->name}'");
        } else {
            Log::warning("BackupDestination: Test failed for '{$destination->name}': ".($result['error'] ?? 'Unknown error'));
        }

        return $result;
    }

    /**
     * Upload a backup to its remote destination.
     */
    public function uploadToRemote(Backup $backup, bool $keepLocal = false): bool
    {
        if (! $backup->destination || ! $backup->local_path) {
            return false;
        }

        try {
            $backup->update(['status' => 'uploading']);

            $config = $this->buildDestinationConfig($backup->destination);
            $backupType = $backup->metadata['backup_type'] ?? 'full';

            $result = $this->agent->backupUploadRemote($backup->local_path, $config, $backupType);

            if ($result['success'] ?? false) {
                $backup->update([
                    'status' => 'completed',
                    'remote_path' => $result['remote_path'] ?? null,
                ]);

                if (! $keepLocal && $backup->local_path) {
                    $this->agent->backupDeleteServer($backup->local_path);
                    $backup->update(['local_path' => null]);
                }

                Log::info("BackupOrchestrator: Uploaded backup {$backup->id} to remote");

                IndexRemoteBackups::dispatch($backup->destination_id);

                return true;
            } else {
                throw new Exception($result['error'] ?? 'Upload failed');
            }
        } catch (Exception $e) {
            $backup->update([
                'status' => 'completed',
                'error_message' => 'Remote upload failed: '.$e->getMessage(),
            ]);

            Log::warning("BackupOrchestrator: Remote upload failed for backup {$backup->id}: ".$e->getMessage());

            return false;
        }
    }

    /**
     * Delete a backup (local file, remote file, and DB record).
     *
     * @throws Exception If a local file path is invalid
     */
    public function deleteBackup(Backup $backup): void
    {
        // Delete local file via agent if exists
        if ($backup->local_path) {
            $this->validatePath($backup->local_path);
            $this->agent->backupDeleteServer($backup->local_path);
        }

        // Delete from remote destination if exists
        if ($backup->remote_path && $backup->destination) {
            try {
                $config = $this->buildDestinationConfig($backup->destination);
                $this->agent->backupDeleteRemote($backup->remote_path, $config);
            } catch (Exception $e) {
                Log::warning('BackupOrchestrator: Failed to delete remote backup: '.$e->getMessage());
            }
        }

        $backup->delete();
    }

    /**
     * Apply retention policy for a schedule, deleting old backups.
     */
    public function applyRetention(BackupSchedule $schedule): int
    {
        $retentionCount = $schedule->retention_count ?? 7;

        $backups = Backup::where('schedule_id', $schedule->id)
            ->where('status', 'completed')
            ->orderByDesc('created_at')
            ->get();

        if ($backups->count() <= $retentionCount) {
            return 0;
        }

        $toDelete = $backups->slice($retentionCount);
        $deletedCount = 0;

        foreach ($toDelete as $oldBackup) {
            Log::info("BackupOrchestrator: Deleting old backup per retention: {$oldBackup->name}");

            try {
                $this->deleteBackup($oldBackup);
                $deletedCount++;
            } catch (Exception $e) {
                Log::warning("BackupOrchestrator: Failed to delete backup {$oldBackup->id} during retention: ".$e->getMessage());
            }
        }

        Log::info("BackupOrchestrator: Deleted {$deletedCount} old backup(s) per retention policy");

        return $deletedCount;
    }

    /**
     * Get the manifest for a backup, optionally filtered to a specific user.
     *
     * @return array<string, mixed>
     */
    public function getBackupManifest(Backup $backup, ?string $forUser = null): array
    {
        $backupPath = $backup->local_path;

        if (! $backupPath || ! file_exists($backupPath)) {
            return [
                'username' => $forUser ?? ($backup->users[0] ?? ''),
                'domains' => $backup->domains ?? [],
                'databases' => $backup->databases ?? [],
                'mailboxes' => $backup->mailboxes ?? [],
                'mysql_users' => $backup->metadata['mysql_users'] ?? [],
                'ssl_certificates' => $backup->ssl_certificates ?? [],
                'dns_zones' => $backup->dns_zones ?? [],
                'users' => $backup->users ?? [],
            ];
        }

        try {
            $result = $this->agent->backupGetInfo($backupPath);

            if ($result['success'] ?? false) {
                $manifest = $result['manifest'] ?? [];

                if (isset($manifest['users']) && is_array($manifest['users']) && $manifest['type'] === 'server') {
                    $userList = array_keys($manifest['users']);

                    if ($forUser === null) {
                        return [
                            'username' => $userList[0] ?? '',
                            'users' => $userList,
                            'type' => 'server',
                            'domains' => $this->aggregateFromUsers($manifest['users'], 'domains'),
                            'databases' => $this->aggregateFromUsers($manifest['users'], 'databases'),
                            'mailboxes' => $this->aggregateFromUsers($manifest['users'], 'mailboxes'),
                            'mysql_users' => [],
                            'ssl_certificates' => [],
                            'dns_zones' => [],
                        ];
                    }

                    if (isset($manifest['users'][$forUser])) {
                        $userData = $manifest['users'][$forUser];

                        return [
                            'username' => $forUser,
                            'users' => $userList,
                            'type' => 'server',
                            'domains' => $userData['domains'] ?? [],
                            'databases' => $userData['databases'] ?? [],
                            'mailboxes' => $userData['mailboxes'] ?? [],
                            'mysql_users' => [],
                            'ssl_certificates' => [],
                            'dns_zones' => [],
                        ];
                    }
                }

                return $manifest;
            }
        } catch (Exception $e) {
            // Fall back to stored data
        }

        return [
            'username' => $forUser ?? ($backup->users[0] ?? ''),
            'domains' => $backup->domains ?? [],
            'databases' => $backup->databases ?? [],
            'mailboxes' => $backup->mailboxes ?? [],
            'mysql_users' => [],
            'ssl_certificates' => [],
            'dns_zones' => [],
            'users' => $backup->users ?? [],
        ];
    }

    /**
     * Aggregate a key from multiple user data arrays.
     *
     * @param  array<string, array<string, mixed>>  $users
     * @return array<int, mixed>
     */
    private function aggregateFromUsers(array $users, string $key): array
    {
        $result = [];
        foreach ($users as $userData) {
            if (isset($userData[$key]) && is_array($userData[$key])) {
                $result = array_merge($result, $userData[$key]);
            }
        }

        return array_unique($result);
    }

    /**
     * Create a user-scoped backup via the agent.
     *
     * @param  array<string, mixed>  $data
     */
    public function createUserBackup(User $user, array $data): Backup
    {
        $timestamp = now()->format('Y-m-d_His');
        $filename = "backup_{$timestamp}.tar.gz";
        $outputPath = "/home/{$user->username}/backups/{$filename}";
        $destinationId = ! empty($data['destination_id']) ? (int) $data['destination_id'] : null;

        $backup = Backup::create([
            'user_id' => $user->id,
            'name' => $data['name'],
            'filename' => $filename,
            'type' => 'full',
            'include_files' => $data['include_files'] ?? true,
            'include_databases' => $data['include_databases'] ?? true,
            'include_mailboxes' => $data['include_mailboxes'] ?? true,
            'destination_id' => $destinationId,
            'status' => 'pending',
            'local_path' => $outputPath,
        ]);

        $backup->update(['status' => 'running', 'started_at' => now()]);

        $result = $this->agent->backupCreate($user->username, $outputPath, [
            'include_files' => $data['include_files'] ?? true,
            'include_databases' => $data['include_databases'] ?? true,
            'include_mailboxes' => $data['include_mailboxes'] ?? true,
            'include_ssl' => $data['include_ssl'] ?? true,
        ]);

        if (! ($result['success'] ?? false)) {
            $backup->update([
                'status' => 'failed',
                'completed_at' => now(),
                'error_message' => $result['error'] ?? 'Backup failed',
            ]);

            throw new Exception($result['error'] ?? 'Backup failed');
        }

        $backup->update([
            'status' => 'completed',
            'completed_at' => now(),
            'size_bytes' => $result['size'] ?? 0,
            'checksum' => $result['checksum'] ?? null,
            'domains' => $result['domains'] ?? null,
            'databases' => $result['databases'] ?? null,
            'mailboxes' => $result['mailboxes'] ?? null,
        ]);

        return $backup;
    }

    /**
     * Restore a user-scoped backup via the agent.
     *
     * @param  array<string, mixed>  $options
     * @return array{success: bool, result: array<string, mixed>, error: string}
     */
    public function restoreBackup(User $user, Backup $backup, array $options): array
    {
        if ($backup->user_id !== $user->id) {
            return ['success' => false, 'result' => [], 'error' => 'Backup not found'];
        }

        $restore = BackupRestore::create([
            'backup_id' => $backup->id,
            'user_id' => $user->id,
            'restore_files' => $options['restore_files'] ?? true,
            'restore_databases' => $options['restore_databases'] ?? true,
            'restore_mailboxes' => $options['restore_mailboxes'] ?? true,
            'selected_domains' => ! empty($options['selected_domains']) ? $options['selected_domains'] : null,
            'selected_databases' => ! empty($options['selected_databases']) ? $options['selected_databases'] : null,
            'selected_mailboxes' => ! empty($options['selected_mailboxes']) ? $options['selected_mailboxes'] : null,
            'status' => 'pending',
        ]);

        try {
            $restore->markAsRunning();

            $result = $this->agent->backupRestore($user->username, $backup->local_path, [
                'restore_files' => $options['restore_files'] ?? true,
                'restore_databases' => $options['restore_databases'] ?? true,
                'restore_mailboxes' => $options['restore_mailboxes'] ?? true,
                'selected_domains' => ! empty($options['selected_domains']) ? $options['selected_domains'] : null,
                'selected_databases' => ! empty($options['selected_databases']) ? $options['selected_databases'] : null,
                'selected_mailboxes' => ! empty($options['selected_mailboxes']) ? $options['selected_mailboxes'] : null,
            ]);

            if ($result['success'] ?? false) {
                $restore->markAsCompleted($result['restored'] ?? []);

                return [
                    'success' => true,
                    'result' => $result,
                    'error' => '',
                ];
            }

            throw new Exception($result['error'] ?? 'Restore failed');
        } catch (Exception $e) {
            $restore->markAsFailed($e->getMessage());

            return [
                'success' => false,
                'result' => [],
                'error' => $e->getMessage(),
            ];
        }
    }

    /**
     * Download a user backup from a remote destination.
     */
    public function downloadFromRemote(User $user, int $destinationId, string $remotePath): Backup
    {
        $destination = BackupDestination::where('id', $destinationId)
            ->where('user_id', $user->id)
            ->first();

        if (! $destination) {
            throw new Exception('Destination not found');
        }

        $pathParts = explode('/', trim($remotePath, '/'));
        $backupDate = $pathParts[0] ?? now()->format('Y-m-d_His');
        $timestamp = now()->format('Y-m-d_His');
        $filename = "backup_{$timestamp}.tar.gz";
        $localPath = "/home/{$user->username}/backups/{$filename}";

        $config = $this->buildDestinationConfig($destination);

        $result = $this->agent->send('backup.download_user_archive', [
            'username' => $user->username,
            'remote_path' => $remotePath,
            'destination' => $config,
            'output_path' => $localPath,
        ]);

        if (! ($result['success'] ?? false)) {
            throw new Exception($result['error'] ?? 'Download failed');
        }

        return Backup::create([
            'user_id' => $user->id,
            'name' => "Server Backup ({$backupDate})",
            'filename' => $filename,
            'type' => 'full',
            'status' => 'completed',
            'local_path' => $localPath,
            'size_bytes' => $result['size'] ?? 0,
            'completed_at' => now(),
        ]);
    }

    /**
     * Test a user-scoped backup destination.
     *
     * @return array<string, mixed>
     */
    public function testUserDestination(User $user, int $destinationId): array
    {
        $destination = BackupDestination::where('id', $destinationId)
            ->where('user_id', $user->id)
            ->first();

        if (! $destination) {
            throw new Exception('Destination not found');
        }

        return $this->testDestination($destination);
    }

    /**
     * Delete a user-scoped backup (local file and DB record).
     */
    public function deleteUserBackup(User $user, Backup $backup): void
    {
        if ($backup->user_id !== $user->id) {
            throw new Exception('Backup not found');
        }

        if ($backup->local_path) {
            try {
                $this->agent->backupDelete($user->username, $backup->local_path);
            } catch (Exception $e) {
                Log::warning("BackupOrchestrator: Failed to delete local file for backup {$backup->id}: ".$e->getMessage());
            }
        }

        $backup->delete();
    }

    /**
     * Validate a backup path is safe to delete.
     *
     * @throws Exception If the path is invalid
     */
    private function validatePath(string $path): void
    {
        if (empty($path) || str_contains($path, '..') || (! str_starts_with($path, '/home/') && ! str_starts_with($path, '/var/backups/'))) {
            throw new Exception("Invalid backup path: {$path}");
        }
    }
}
