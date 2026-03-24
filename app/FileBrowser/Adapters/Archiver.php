<?php

declare(strict_types=1);

namespace App\FileBrowser\Adapters;

interface Archiver
{
    public function extract(string $path): array;
}
