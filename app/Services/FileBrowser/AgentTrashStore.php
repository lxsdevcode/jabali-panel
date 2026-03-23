<?php

declare(strict_types=1);

namespace App\Services\FileBrowser;

use App\Services\Agent\AgentClient;
use Jabali\FileBrowser\Services\TrashStore;

/**
 * TrashStore that delegates to the Jabali agent daemon.
 *
 * The agent handles trash operations natively (file.trash, file.restore,
 * file.list_trash, file.empty_trash) as root, which is required because
 * PHP runs as www-data and cannot access user home directories directly.
 */
class AgentTrashStore implements TrashStore
{
    public function __construct(
        private readonly AgentClient $agent,
        private readonly string $username,
    ) {}

    public function recordTrash(string $trashName, array $meta): void
    {
        // The agent handles trash metadata internally via file.trash.
        // This is called by TrashManager before move(), but for the agent
        // we let the agent's file.trash handle everything. We store the
        // path so TrashManager can call the agent's trash action.
    }

    public function readMeta(string $trashName): array
    {
        // Individual metadata is returned as part of listTrashed items.
        // The agent doesn't expose per-item metadata reads.
        $items = $this->listAllItems();
        foreach ($items as $item) {
            if (($item['trash_name'] ?? '') === $trashName) {
                return [
                    'original_path' => $item['original_path'] ?? '',
                    'trashed_at' => $item['trashed_at'] ?? 0,
                    'name' => $item['name'] ?? $trashName,
                ];
            }
        }

        return [];
    }

    public function forgetTrash(string $trashName): void
    {
        // Agent handles cleanup internally on restore/delete.
    }

    /**
     * Trash a file via the agent daemon.
     */
    public function agentTrash(string $path): array
    {
        return $this->agent->fileTrash($this->username, $path);
    }

    /**
     * Restore a file via the agent daemon.
     */
    public function agentRestore(string $trashName): array
    {
        return $this->agent->fileRestore($this->username, $trashName);
    }

    /**
     * Empty trash via the agent daemon.
     */
    public function agentEmptyTrash(): array
    {
        return $this->agent->fileEmptyTrash($this->username);
    }

    /**
     * List all trash items via the agent daemon.
     */
    public function agentListTrash(): array
    {
        $result = $this->agent->fileListTrash($this->username);

        return $result['items'] ?? [];
    }

    private function listAllItems(): array
    {
        return $this->agentListTrash();
    }
}
