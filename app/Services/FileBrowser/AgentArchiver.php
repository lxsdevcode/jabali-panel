<?php

declare(strict_types=1);

namespace App\Services\FileBrowser;

use App\FileBrowser\Adapters\Archiver;
use App\Services\Agent\AgentClient;

class AgentArchiver implements Archiver
{
    public function __construct(
        private AgentClient $agent,
        private string $username,
    ) {}

    public function extract(string $path): array
    {
        return $this->agent->fileExtract($this->username, $path);
    }
}
