<?php

declare(strict_types=1);

namespace App\Services\FileBrowser;

use App\FileBrowser\Adapters\PermissionManager;
use App\Services\Agent\AgentClient;

class AgentPermissionManager implements PermissionManager
{
    public function __construct(
        private AgentClient $agent,
        private string $username,
    ) {}

    public function chmod(string $path, string $mode): array
    {
        return $this->agent->fileChmod($this->username, $path, $mode);
    }
}
