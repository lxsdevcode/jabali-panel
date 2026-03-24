<?php

declare(strict_types=1);

namespace App\FileBrowser\Adapters;

interface PermissionManager
{
    public function chmod(string $path, string $mode): array;
}
