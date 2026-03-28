<?php

declare(strict_types=1);

namespace App\Services\Agent;

class FileAgentService
{
    public function __construct(private AgentClient $agent) {}

    public function list(string $username, string $path, bool $showHidden = false): array
    {
        return $this->agent->fileList($username, $path, $showHidden);
    }

    public function read(string $username, string $path): array
    {
        return $this->agent->fileRead($username, $path);
    }

    public function write(string $username, string $path, string $content): array
    {
        return $this->agent->fileWrite($username, $path, $content);
    }

    public function delete(string $username, string $path): array
    {
        return $this->agent->fileDelete($username, $path);
    }

    public function mkdir(string $username, string $path): array
    {
        return $this->agent->fileMkdir($username, $path);
    }

    public function rename(string $username, string $oldPath, string $newPath): array
    {
        return $this->agent->fileRename($username, $oldPath, $newPath);
    }

    public function copy(string $username, string $source, string $destination): array
    {
        return $this->agent->fileCopy($username, $source, $destination);
    }

    public function move(string $username, string $source, string $destination): array
    {
        return $this->agent->fileMove($username, $source, $destination);
    }

    public function upload(string $username, string $path, string $filename, string $content): array
    {
        return $this->agent->fileUpload($username, $path, $filename, $content);
    }

    public function extract(string $username, string $path): array
    {
        return $this->agent->fileExtract($username, $path);
    }

    public function chmod(string $username, string $path, string $mode): array
    {
        return $this->agent->fileChmod($username, $path, $mode);
    }

    public function info(string $username, string $path): array
    {
        return $this->agent->fileInfo($username, $path);
    }

    public function trash(string $username, string $path): array
    {
        return $this->agent->fileTrash($username, $path);
    }

    public function restore(string $username, string $trashName): array
    {
        return $this->agent->fileRestore($username, $trashName);
    }

    public function emptyTrash(string $username): array
    {
        return $this->agent->fileEmptyTrash($username);
    }

    public function listTrash(string $username): array
    {
        return $this->agent->fileListTrash($username);
    }

    public function imageOptimize(string $username, string $path, bool $convertWebp = false, int $quality = 82): array
    {
        return $this->agent->imageOptimize($username, $path, $convertWebp, $quality);
    }
}
