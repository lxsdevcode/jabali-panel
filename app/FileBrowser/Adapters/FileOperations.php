<?php

declare(strict_types=1);

namespace App\FileBrowser\Adapters;

interface FileOperations
{
    /** @return array{items: array<int, array{name: string, path: string, is_dir: bool, size: int|null, modified: int, permissions: string}>} */
    public function list(string $path, bool $showHidden = false): array;

    /** @return array{content: string} */
    public function read(string $path): array;

    public function write(string $path, string $content): array;

    public function delete(string $path): array;

    public function mkdir(string $path): array;

    public function rename(string $oldPath, string $newPath): array;

    public function copy(string $source, string $destination): array;

    public function move(string $source, string $destination): array;

    public function upload(string $directory, string $filename, string $content): array;

    /** @return array{info: array{permissions: string}} */
    public function info(string $path): array;
}
