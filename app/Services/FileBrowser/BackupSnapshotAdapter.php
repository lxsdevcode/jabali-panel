<?php

declare(strict_types=1);

namespace App\Services\FileBrowser;

use App\FileBrowser\Adapters\Archiver;
use App\FileBrowser\Adapters\FileBrowserAdapter;
use App\FileBrowser\Adapters\FileOperations;
use App\FileBrowser\Adapters\PermissionManager;
use App\Services\Agent\AgentClient;

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
