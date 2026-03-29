<?php

declare(strict_types=1);

namespace App\FileBrowser\Services;

class TrashResult
{
    private function __construct(
        public readonly bool $success,
        public readonly string $message,
        public readonly array $data,
    ) {}

    public static function success(string $message, array $data = []): self
    {
        return new self(success: true, message: $message, data: $data);
    }

    public static function failure(string $message, array $data = []): self
    {
        return new self(success: false, message: $message, data: $data);
    }

    public function restoredPath(): string
    {
        return $this->data['restored_path'] ?? '';
    }

    public function deletedCount(): int
    {
        return $this->data['deleted'] ?? 0;
    }
}
