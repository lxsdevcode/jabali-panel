<?php

declare(strict_types=1);

namespace App\Services\FileBrowser;

use App\FileBrowser\Adapters\FileOperations;
use App\Services\Agent\AgentClient;
use RuntimeException;

/**
 * Read-only file operations for browsing Restic backup snapshots.
 * Uses restic ls via the agent to list snapshot contents.
 * Write operations throw — snapshots are immutable.
 */
class BackupSnapshotFileOperations implements FileOperations
{
    public function __construct(
        private AgentClient $agent,
        private string $snapshotId,
        private string $username,
        private string $repo,
        private array $destinationConfig = [],
    ) {}

    public function list(string $path, bool $showHidden = false): array
    {
        $fullPath = "/home/{$this->username}";
        if (! empty($path)) {
            $fullPath .= '/'.ltrim($path, '/');
        }

        $result = $this->agent->send('backup.list_domain_files', [
            'snapshot_id' => $this->snapshotId,
            'username' => $this->username,
            'path' => $path,
            'repo' => $this->repo,
            'destination' => $this->destinationConfig,
        ]);

        $items = [];

        // Add parent navigation if not at root
        if (! empty($path)) {
            $parentPath = dirname($path);
            if ($parentPath === '.') {
                $parentPath = '';
            }
            $items[] = [
                'name' => '..',
                'path' => $parentPath,
                'is_dir' => true,
                'size' => null,
                'modified' => time(),
                'permissions' => '0755',
                'is_parent' => true,
            ];
        }

        // restic ls returns the directory entry itself alongside children — filter it out
        $currentDirName = ! empty($path) ? basename($path) : $this->username;

        foreach ($result['items'] ?? [] as $file) {
            $name = $file['name'] ?? '';
            if ($name === '.' || $name === '..' || empty($name)) {
                continue;
            }

            // Skip the directory entry that matches the current path (restic ls includes it)
            if ($name === $currentDirName && ($file['is_dir'] ?? false)) {
                continue;
            }

            if (! $showHidden && str_starts_with($name, '.')) {
                continue;
            }

            $filePath = empty($path) ? $name : $path.'/'.$name;

            $items[] = [
                'name' => $name,
                'path' => $filePath,
                'is_dir' => $file['is_dir'] ?? false,
                'size' => ($file['is_dir'] ?? false) ? null : ($file['size'] ?? null),
                'modified' => $file['modified'] ?? time(),
                'permissions' => $file['permissions'] ?? '0644',
            ];
        }

        return ['items' => $items];
    }

    public function read(string $path): array
    {
        throw new RuntimeException('Backup snapshots are read-only');
    }

    public function write(string $path, string $content): array
    {
        throw new RuntimeException('Backup snapshots are read-only');
    }

    public function delete(string $path): array
    {
        throw new RuntimeException('Backup snapshots are read-only');
    }

    public function mkdir(string $path): array
    {
        throw new RuntimeException('Backup snapshots are read-only');
    }

    public function rename(string $oldPath, string $newPath): array
    {
        throw new RuntimeException('Backup snapshots are read-only');
    }

    public function copy(string $source, string $destination): array
    {
        throw new RuntimeException('Backup snapshots are read-only');
    }

    public function move(string $source, string $destination): array
    {
        throw new RuntimeException('Backup snapshots are read-only');
    }

    public function upload(string $directory, string $filename, string $content): array
    {
        throw new RuntimeException('Backup snapshots are read-only');
    }

    public function info(string $path): array
    {
        return ['info' => ['permissions' => '0644']];
    }
}
