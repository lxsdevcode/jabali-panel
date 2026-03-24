<?php

declare(strict_types=1);

namespace App\FileBrowser\Services;

interface TrashStore
{
    public function recordTrash(string $trashName, array $meta): void;

    public function readMeta(string $trashName): array;

    public function forgetTrash(string $trashName): void;
}
