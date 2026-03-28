<?php

declare(strict_types=1);

namespace App\Services\FileBrowser;

use App\FileBrowser\Adapters\Archiver;
use App\FileBrowser\Adapters\FileBrowserAdapter;
use App\FileBrowser\Adapters\FileOperations;
use App\FileBrowser\Adapters\PermissionManager;
use App\Models\BackupDestination;
use App\Services\Agent\AgentClient;

/**
 * Read-only adapter for browsing Restic backup snapshots.
 * Uses restic ls via the agent to list snapshot contents.
 */
class BackupSnapshotAdapter implements FileBrowserAdapter
{
    private BackupSnapshotFileOperations $fileOps;

    public function __construct(
        AgentClient $agent,
        string $snapshotId,
        string $username,
        string $repo = '',
        array $destinationConfig = [],
    ) {
        $this->fileOps = new BackupSnapshotFileOperations(
            $agent,
            $snapshotId,
            $username,
            $repo ?: BackupDestination::defaultRepo(),
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
