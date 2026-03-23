<?php

declare(strict_types=1);

namespace App\Services\FileBrowser;

use App\Services\Agent\AgentClient;
use Jabali\FileBrowser\Adapters\FileOperations;
use RuntimeException;

/**
 * Read-only file operations for browsing backup snapshots via SSH.
 * Write operations throw — snapshots are immutable.
 * All SSH args are escaped with escapeshellarg() per project conventions.
 */
class BackupSnapshotFileOperations implements FileOperations
{
    public function __construct(
        private AgentClient $agent,
        private string $backupPath,
        private string $username,
        private array $destinationConfig,
    ) {}

    public function list(string $path, bool $showHidden = false): array
    {
        $remotePath = rtrim($this->backupPath, '/')
            .'/'.$this->username
            .'/home/'.$this->username;

        if (! empty($path)) {
            $remotePath .= '/'.ltrim($path, '/');
        }

        // Use the agent to list the remote directory via SSH
        $result = $this->agent->send('backup.list_snapshot_dir', [
            'remote_path' => $remotePath,
            'destination' => $this->destinationConfig,
            'show_hidden' => $showHidden,
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

        foreach ($result['items'] ?? [] as $file) {
            $name = $file['name'] ?? '';
            if ($name === '.' || $name === '..' || empty($name)) {
                continue;
            }

            $filePath = empty($path) ? $name : $path.'/'.$name;

            $items[] = [
                'name' => $name,
                'path' => $filePath,
                'is_dir' => $file['is_dir'] ?? false,
                'size' => ($file['is_dir'] ?? false) ? null : ($file['size'] ?? null),
                'modified' => $file['mtime'] ?? $file['modified'] ?? time(),
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
