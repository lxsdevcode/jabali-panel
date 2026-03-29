<?php

declare(strict_types=1);

namespace App\Services\FileBrowser;

use App\FileBrowser\Adapters\FileOperations;
use App\Services\Agent\AgentClient;

class AgentFileOperations implements FileOperations
{
    public function __construct(
        private AgentClient $agent,
        private string $username,
    ) {}

    public function list(string $path, bool $showHidden = false): array
    {
        // Reject path traversal attempts at the adapter level
        if ($path !== '' && preg_match('/(?:^|[\\/])\.\.([\\/]|$)/', $path)) {
            return ['items' => []];
        }

        try {
            $result = $this->agent->fileList($this->username, $path, $showHidden);
        } catch (\Exception $e) {
            return ['items' => []];
        }

        if (! ($result['success'] ?? false)) {
            return ['items' => []];
        }

        $items = [];
        foreach ($result['items'] ?? $result['files'] ?? [] as $file) {
            $name = $file['name'] ?? '';
            if ($name === '' || $name === '.' || $name === '..') {
                continue;
            }

            $items[] = [
                'name' => $name,
                'path' => $file['path'] ?? (empty($path) ? $name : $path.'/'.$name),
                'is_dir' => ($file['type'] ?? '') === 'directory' || ($file['is_dir'] ?? false),
                'size' => $file['size'] ?? null,
                'modified' => $file['mtime'] ?? $file['modified'] ?? time(),
                'permissions' => $file['permissions'] ?? '0644',
            ];
        }

        // Sort: directories first, then files — alphabetical within each group
        usort($items, function (array $a, array $b): int {
            if (($a['is_dir'] ?? false) !== ($b['is_dir'] ?? false)) {
                return ($a['is_dir'] ?? false) ? -1 : 1;
            }

            return strcasecmp($a['name'], $b['name']);
        });

        return ['items' => $items];
    }

    public function read(string $path): array
    {
        $result = $this->agent->fileRead($this->username, $path);

        if (! ($result['success'] ?? false)) {
            throw new \RuntimeException($result['error'] ?? 'Failed to read file');
        }

        $content = $result['content'] ?? '';
        if (($result['encoding'] ?? '') === 'base64') {
            // Content is already base64 from the agent — return as-is
            return ['content' => $content];
        }

        // Content is plain text — base64 encode for the interface contract
        return ['content' => base64_encode($content)];
    }

    public function write(string $path, string $content): array
    {
        return $this->agent->fileWrite($this->username, $path, $content);
    }

    public function delete(string $path): array
    {
        return $this->agent->fileDelete($this->username, $path);
    }

    public function mkdir(string $path): array
    {
        return $this->agent->fileMkdir($this->username, $path);
    }

    public function rename(string $oldPath, string $newPath): array
    {
        return $this->agent->fileRename($this->username, $oldPath, $newPath);
    }

    public function copy(string $source, string $destination): array
    {
        return $this->agent->fileCopy($this->username, $source, $destination);
    }

    public function move(string $source, string $destination): array
    {
        return $this->agent->fileMove($this->username, $source, $destination);
    }

    public function upload(string $directory, string $filename, string $content): array
    {
        return $this->agent->fileUpload($this->username, $directory, $filename, $content);
    }

    public function info(string $path): array
    {
        return $this->agent->fileInfo($this->username, $path);
    }
}
