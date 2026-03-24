<?php

declare(strict_types=1);

namespace App\Services\FileBrowser;

use App\FileBrowser\Adapters\Archiver;
use App\FileBrowser\Adapters\FileBrowserAdapter;
use App\FileBrowser\Adapters\FileOperations;
use App\FileBrowser\Adapters\PermissionManager;
use App\Services\Agent\AgentClient;

class AgentAdapter implements FileBrowserAdapter
{
    private AgentFileOperations $fileOps;

    private AgentPermissionManager $permissionMgr;

    private AgentArchiver $archiver;

    public function __construct(
        private AgentClient $agent,
        private string $username,
    ) {
        $this->fileOps = new AgentFileOperations($agent, $username);
        $this->permissionMgr = new AgentPermissionManager($agent, $username);
        $this->archiver = new AgentArchiver($agent, $username);
    }

    public function files(): FileOperations
    {
        return $this->fileOps;
    }

    public function archiver(): ?Archiver
    {
        return $this->archiver;
    }

    public function permissions(): ?PermissionManager
    {
        return $this->permissionMgr;
    }
}
