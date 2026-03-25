<?php

declare(strict_types=1);

namespace App\Services\Backup;

use App\Models\BackupRestore;
use App\Models\User;
use App\Models\UserRemoteBackup;
use App\Services\Agent\AgentClientInterface;
use Illuminate\Support\Collection;
use RuntimeException;

class GranularRestoreService
{
    public function __construct(private AgentClientInterface $agent) {}

    /**
     * List available backup snapshots for a user, sorted newest-first.
     *
     * @return Collection<int, UserRemoteBackup>
     */
    public function listSnapshots(User $user): Collection
    {
        return UserRemoteBackup::forUser($user->id)
            ->orderByDesc('backup_date')
            ->get();
    }

    /**
     * Perform a full account restore from a snapshot.
     */
    public function fullRestore(User $user, UserRemoteBackup $snapshot, bool $safetyBackup = false, ?User $performedBy = null): BackupRestore
    {
        $this->validateSnapshotPath($snapshot->backup_path);
        $this->authorizeRestore($user, $snapshot, $performedBy);

        if ($safetyBackup) {
            $this->agent->send('backup.create', [
                'username' => $user->username,
            ]);
        }

        $restore = BackupRestore::create([
            'snapshot_id' => $snapshot->id,
            'user_id' => $user->id,
            'performed_by' => $performedBy?->id,
            'restore_files' => true,
            'restore_databases' => true,
            'restore_mailboxes' => true,
            'restore_dns' => true,
            'status' => 'pending',
        ]);

        $restore->markAsRunning();

        $result = $this->agent->send('backup.restore', [
            'username' => $user->username,
            'backup_path' => $snapshot->backup_path,
            'restore_files' => true,
            'restore_databases' => true,
            'restore_mailboxes' => true,
            'restore_dns' => true,
            'restore_ssl' => true,
            'restore_mysql_users' => true,
        ]);

        if ($result['success'] ?? false) {
            $restore->markAsCompleted([
                'restored' => $result['restored'] ?? [],
                'failed' => $result['failed'] ?? [],
            ]);
        } else {
            $restore->markAsFailed($result['error'] ?? 'Restore failed');
        }

        return $restore->fresh();
    }

    /**
     * Perform a selective restore of specific items from a snapshot.
     *
     * @param  array{domains?: list<string>, databases?: list<string>, mailboxes?: list<string>, mysql_users?: bool, dns_zones?: list<string>, ssl_certificates?: list<string>, files?: list<string>}  $selection
     * @param  array{databases?: string, mailboxes?: string, files?: string, mysql_users?: string}  $conflictResolution
     */
    public function selectiveRestore(User $user, UserRemoteBackup $snapshot, array $selection, array $conflictResolution = []): BackupRestore
    {
        $this->validateSnapshotPath($snapshot->backup_path);

        $hasDomains = ! empty($selection['domains']) || ! empty($selection['files']);
        $hasDatabases = ! empty($selection['databases']);
        $hasMailboxes = ! empty($selection['mailboxes']);
        $hasMysqlUsers = ! empty($selection['mysql_users']);
        $hasDns = ! empty($selection['dns_zones']);
        $hasSsl = ! empty($selection['ssl_certificates']);

        $restore = BackupRestore::create([
            'snapshot_id' => $snapshot->id,
            'user_id' => $user->id,
            'restore_files' => $hasDomains,
            'restore_databases' => $hasDatabases,
            'restore_mailboxes' => $hasMailboxes,
            'restore_dns' => $hasDns,
            'restore_ssl' => $hasSsl,
            'restore_mysql_users' => $hasMysqlUsers,
            'selected_domains' => $selection['domains'] ?? null,
            'selected_databases' => $selection['databases'] ?? null,
            'selected_mailboxes' => $selection['mailboxes'] ?? null,
            'status' => 'pending',
        ]);

        $restore->markAsRunning();

        $agentParams = [
            'username' => $user->username,
            'backup_path' => $snapshot->backup_path,
            'restore_files' => $hasDomains,
            'restore_databases' => $hasDatabases,
            'restore_mailboxes' => $hasMailboxes,
            'restore_dns' => $hasDns,
            'restore_ssl' => $hasSsl,
            'restore_mysql_users' => $hasMysqlUsers,
        ];

        if ($hasDatabases) {
            $agentParams['selected_databases'] = $selection['databases'];
        }
        if ($hasMailboxes) {
            $agentParams['selected_mailboxes'] = $selection['mailboxes'];
        }
        if (! empty($selection['domains'])) {
            $agentParams['selected_domains'] = $selection['domains'];
        }
        if (! empty($selection['files'])) {
            $agentParams['selected_files'] = $selection['files'];
        }
        if ($hasDns) {
            $agentParams['selected_dns_zones'] = $selection['dns_zones'];
        }
        if ($hasSsl) {
            $agentParams['selected_ssl_certificates'] = $selection['ssl_certificates'];
        }

        if (! empty($conflictResolution)) {
            $agentParams['conflict_resolution'] = $conflictResolution;
        }

        $result = $this->agent->send('backup.restore', $agentParams);

        if ($result['success'] ?? false) {
            $restore->markAsCompleted([
                'restored' => $result['restored'] ?? [],
                'failed' => $result['failed'] ?? [],
            ]);
        } else {
            $restore->markAsFailed($result['error'] ?? 'Restore failed');
        }

        return $restore->fresh();
    }

    /**
     * Browse a snapshot's contents, categorized by resource type.
     *
     * @return array{domains: list<string>, databases: list<string>, mailboxes: list<string>, mysql_users: bool, dns_zones: list<string>, ssl_certificates: list<string>}
     */
    public function browseSnapshot(User $user, UserRemoteBackup $snapshot): array
    {
        $this->validateSnapshotPath($snapshot->backup_path);

        $destination = $snapshot->destination;
        $destConfig = $destination
            ? array_merge($destination->config ?? [], ['type' => $destination->type])
            : [];

        $result = $this->agent->send('backup.list_snapshot_tree', [
            'backup_path' => $snapshot->backup_path,
            'username' => $user->username,
            'destination' => $destConfig,
        ]);

        $files = $result['files'] ?? [];

        return $this->categorizeSnapshotFiles($files);
    }

    /**
     * Parse flat file list into categorized restore items.
     *
     * @param  list<string>  $files
     * @return array{domains: list<string>, databases: list<string>, mailboxes: list<string>, mysql_users: bool, dns_zones: list<string>, ssl_certificates: list<string>}
     */
    private function categorizeSnapshotFiles(array $files): array
    {
        $domains = [];
        $databases = [];
        $mailboxes = [];
        $mysqlUsers = false;
        $dnsZones = [];
        $sslCertificates = [];

        foreach ($files as $file) {
            $parts = explode('/', $file);

            if ($parts[0] === 'domains' && count($parts) === 2) {
                $domains[] = $parts[1];
            } elseif ($parts[0] === 'mysql' && count($parts) === 2) {
                if ($parts[1] === 'users.sql') {
                    $mysqlUsers = true;
                } elseif (str_ends_with($parts[1], '.sql')) {
                    $databases[] = $parts[1];
                }
            } elseif ($parts[0] === 'mail' && count($parts) === 3) {
                $mailboxes[] = $parts[1].'/'.$parts[2];
            } elseif ($parts[0] === 'zones' && count($parts) === 2) {
                $dnsZones[] = $parts[1];
            } elseif ($parts[0] === 'ssl' && count($parts) === 2) {
                $sslCertificates[] = $parts[1];
            }
        }

        return [
            'domains' => $domains,
            'databases' => $databases,
            'mailboxes' => $mailboxes,
            'mysql_users' => $mysqlUsers,
            'dns_zones' => $dnsZones,
            'ssl_certificates' => $sslCertificates,
        ];
    }

    /**
     * Browse files within a domain directory in a snapshot (lazy-load).
     *
     * @return list<array{name: string, is_dir: bool, path: string, size: int, permissions: string, modified: int}>
     */
    public function browseDomainFiles(User $user, UserRemoteBackup $snapshot, string $relativePath): array
    {
        $this->validateSnapshotPath($snapshot->backup_path);
        $this->validateSnapshotPath($relativePath);

        $destination = $snapshot->destination;
        $destConfig = $destination
            ? array_merge($destination->config ?? [], ['type' => $destination->type])
            : [];

        $remotePath = rtrim($snapshot->backup_path, '/').'/'.$user->username.'/home/'.$user->username.'/'.$relativePath;

        $result = $this->agent->send('backup.list_snapshot_dir', [
            'remote_path' => $remotePath,
            'destination' => $destConfig,
        ]);

        return $result['items'] ?? [];
    }

    /**
     * Validate a snapshot path is safe to use.
     *
     * @throws RuntimeException If path contains traversal sequences
     */
    /**
     * Check that the user is authorized to restore from this snapshot.
     *
     * @throws RuntimeException If user doesn't own the snapshot and is not admin
     */
    private function authorizeRestore(User $user, UserRemoteBackup $snapshot, ?User $performedBy = null): void
    {
        $actor = $performedBy ?? $user;

        if ($snapshot->user_id !== $user->id && ! $actor->is_admin) {
            throw new RuntimeException('Unauthorized: snapshot belongs to another user');
        }
    }

    /**
     * Validate a snapshot path is safe to use.
     *
     * @throws RuntimeException If path contains traversal sequences
     */
    private function validateSnapshotPath(string $path): void
    {
        if (str_contains($path, '..') || str_starts_with($path, '/')) {
            throw new RuntimeException('Invalid snapshot path');
        }
    }
}
