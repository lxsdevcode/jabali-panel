<?php

declare(strict_types=1);

namespace App\Services\Agent;

class BackupAgentService
{
    public function __construct(private AgentClient $agent) {}

    public function create(string $username, string $outputPath, array $options = []): array
    {
        return $this->agent->backupCreate($username, $outputPath, $options);
    }

    public function restore(string $username, string $backupPath, array $options = []): array
    {
        return $this->agent->backupRestore($username, $backupPath, $options);
    }

    public function delete(string $username, string $backupPath): array
    {
        return $this->agent->backupDelete($username, $backupPath);
    }

    public function list(string $username, string $path = ''): array
    {
        return $this->agent->backupList($username, $path);
    }

    public function listContents(string $backupPath, ?string $username = null, ?array $destination = null): array
    {
        return $this->agent->backupListContents($backupPath, $username, $destination);
    }

    public function getInfo(string $backupPath): array
    {
        return $this->agent->backupGetInfo($backupPath);
    }

    public function verify(string $backupPath): array
    {
        return $this->agent->backupVerify($backupPath);
    }

    public function testDestination(array $destination): array
    {
        return $this->agent->backupTestDestination($destination);
    }

    public function createServer(string $outputPath, array $options = []): array
    {
        return $this->agent->backupCreateServer($outputPath, $options);
    }

    public function deleteServer(string $backupPath): array
    {
        return $this->agent->backupDeleteServer($backupPath);
    }

    public function listRemote(array $destination, string $path = ''): array
    {
        return $this->agent->backupListRemote($destination, $path);
    }

    public function uploadRemote(string $localPath, array $destination, string $backupType = 'full'): array
    {
        return $this->agent->backupUploadRemote($localPath, $destination, $backupType);
    }

    public function downloadRemote(string $remotePath, string $localPath, array $destination): array
    {
        return $this->agent->backupDownloadRemote($remotePath, $localPath, $destination);
    }

    public function deleteRemote(string $remotePath, array $destination): array
    {
        return $this->agent->backupDeleteRemote($remotePath, $destination);
    }

    public function incrementalDirect(array $destination, array $options = []): array
    {
        return $this->agent->backupIncrementalDirect($destination, $options);
    }
}
