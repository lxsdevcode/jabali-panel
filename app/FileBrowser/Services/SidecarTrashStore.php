<?php

declare(strict_types=1);

namespace App\FileBrowser\Services;

use App\FileBrowser\Adapters\FileBrowserAdapter;
use RuntimeException;

class SidecarTrashStore implements TrashStore
{
    private const DIR = '.trash';

    public function __construct(
        private readonly FileBrowserAdapter $adapter,
    ) {}

    public function recordTrash(string $trashName, array $meta): void
    {
        $json = json_encode($meta, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES);
        $this->adapter->files()->write(self::DIR.'/'.$trashName.'.json', $json);
    }

    public function readMeta(string $trashName): array
    {
        try {
            $result = $this->adapter->files()->read(self::DIR.'/'.$trashName.'.json');
            $decoded = base64_decode($result['content'] ?? '', true);
            if ($decoded === false) {
                return [];
            }
            $data = json_decode($decoded, true);

            return is_array($data) ? $data : [];
        } catch (RuntimeException) {
            return [];
        }
    }

    public function forgetTrash(string $trashName): void
    {
        try {
            $this->adapter->files()->delete(self::DIR.'/'.$trashName.'.json');
        } catch (RuntimeException) {
            // best-effort
        }
    }
}
