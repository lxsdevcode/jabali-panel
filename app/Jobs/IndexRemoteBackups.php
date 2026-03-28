<?php

declare(strict_types=1);

namespace App\Jobs;

use App\Models\BackupDestination;
use App\Models\User;
use App\Models\UserRemoteBackup;
use App\Services\Agent\AgentClient;
use Exception;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Queue\Queueable;
use Illuminate\Support\Facades\Log;

class IndexRemoteBackups implements ShouldQueue
{
    use Queueable;

    public int $tries = 1;

    public int $timeout = 600;

    public function __construct(
        public ?int $destinationId = null
    ) {}

    public function handle(): void
    {
        $agent = app(AgentClient::class);

        $query = BackupDestination::where('is_server_backup', true)->where('is_active', true);
        if ($this->destinationId) {
            $query->where('id', $this->destinationId);
        }
        $destinations = $query->get();

        $users = User::pluck('id', 'username')->toArray();

        foreach ($destinations as $destination) {
            try {
                $this->indexDestination($agent, $destination, $users);
            } catch (Exception $e) {
                Log::warning("IndexRemoteBackups: Failed to index destination {$destination->name}: ".$e->getMessage());
            }
        }

        Log::info('IndexRemoteBackups: Indexing completed');
    }

    protected function indexDestination(AgentClient $agent, BackupDestination $destination, array $users): void
    {
        $repo = $destination->getResticRepoUrl();

        // Query Restic for all snapshots in this repo
        $result = $agent->send('backup.list_snapshots', [
            'repo' => $repo,
        ]);

        if (! ($result['success'] ?? false) || empty($result['snapshots'])) {
            return;
        }

        $indexedCount = 0;
        $knownSnapshotIds = [];

        foreach ($result['snapshots'] as $snapshot) {
            $snapshotId = $snapshot['short_id'] ?? $snapshot['id'] ?? null;
            if (! $snapshotId) {
                continue;
            }

            $knownSnapshotIds[] = $snapshotId;

            // Extract username from tags (tag format: "user:username")
            $username = null;
            foreach ($snapshot['tags'] ?? [] as $tag) {
                if (str_starts_with($tag, 'user:')) {
                    $username = substr($tag, 5);
                    break;
                }
            }

            if (! $username || ! isset($users[$username])) {
                continue;
            }

            $backupDate = isset($snapshot['time'])
                ? \Carbon\Carbon::parse($snapshot['time'])
                : now();

            UserRemoteBackup::updateOrCreate(
                [
                    'user_id' => $users[$username],
                    'destination_id' => $destination->id,
                    'backup_name' => $snapshotId,
                ],
                [
                    'backup_path' => $snapshotId,
                    'backup_type' => 'restic',
                    'backup_date' => $backupDate,
                    'indexed_at' => now(),
                ]
            );

            $indexedCount++;
        }

        Log::info("IndexRemoteBackups: Indexed {$indexedCount} Restic snapshots from {$destination->name}");

        // Clean up entries for snapshots that no longer exist
        if (! empty($knownSnapshotIds)) {
            $deleted = UserRemoteBackup::where('destination_id', $destination->id)
                ->whereNotIn('backup_name', $knownSnapshotIds)
                ->delete();

            if ($deleted > 0) {
                Log::info("IndexRemoteBackups: Cleaned up {$deleted} deleted snapshot entries");
            }
        }
    }
}
