<?php

declare(strict_types=1);

namespace App\Services\FileBrowser;

use App\Services\Agent\AgentClient;
use Jabali\FileBrowser\Adapters\Archiver;
use Jabali\FileBrowser\Adapters\FileBrowserAdapter;
use Jabali\FileBrowser\Adapters\FileOperations;
use Jabali\FileBrowser\Adapters\PermissionManager;

/**
 * Read-only adapter for browsing backup snapshots on remote destinations.
 * Uses the agent's SSH capabilities to list files on SFTP backup servers.
 */
class BackupSnapshotAdapter implements FileBrowserAdapter
{
    private BackupSnapshotFileOperations $fileOps;

    public function __construct(
        private AgentClient $agent,
        private string $backupPath,
        private string $username,
        private array $destinationConfig,
    ) {
        $this->fileOps = new BackupSnapshotFileOperations(
            $agent,
            $backupPath,
            $username,
            $destinationConfig,
        );
    }

    public function files(): FileOperations
    {
        return $this->fileOps;
    }

    public function archiver(): ?Archiver
    {
        return null;
    }

    public function permissions(): ?PermissionManager
    {
        return null;
    }
}
