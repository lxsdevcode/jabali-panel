<?php

declare(strict_types=1);

namespace App\Services\Migration;

use App\Jobs\RunCpanelRestore;
use App\Models\User;
use App\Services\Agent\AgentClient;
use Exception;
use Illuminate\Support\Facades\Cache;
use Illuminate\Support\Facades\File;
use Illuminate\Support\Str;

class CpanelMigrationOrchestrator
{
    public function __construct(
        private AgentClient $agent,
    ) {}

    /**
     * Test connection to a cPanel server and fetch account summary.
     *
     * @return array{connected: bool, summary: array<string, mixed>, connectionInfo: array<string, int>}
     *
     * @throws Exception
     */
    public function testConnection(string $hostname, string $username, string $apiToken, int $port, bool $useSSL): array
    {
        $cpanel = new CpanelApiService($hostname, $username, $apiToken, $port, $useSSL);

        $result = $cpanel->testConnection();

        if (! $result['success']) {
            throw new Exception($result['message'] ?? __('Connection failed'));
        }

        $summary = $cpanel->getMigrationSummary();

        $connectionInfo = [
            'domains' => $this->countDomains($summary['domains'] ?? []),
            'emails' => count($summary['email_accounts'] ?? []),
            'databases' => count($summary['databases'] ?? []),
            'ssl' => count($summary['ssl_certificates'] ?? []),
        ];

        return [
            'connected' => true,
            'summary' => $summary,
            'connectionInfo' => $connectionInfo,
        ];
    }

    /**
     * Initiate a full backup on the cPanel server.
     *
     * @return array{success: bool, pid: string|null, message: string|null}
     *
     * @throws Exception
     */
    public function startBackup(CpanelApiService $cpanel): array
    {
        $result = $cpanel->createBackup();

        if (! ($result['success'] ?? false)) {
            throw new Exception($result['message'] ?? __('Failed to start backup'));
        }

        return $result;
    }

    /**
     * Check the status of a backup on the cPanel server.
     *
     * @return array{success: bool, backups: array<int, array<string, mixed>>, in_progress: bool}
     *
     * @throws Exception
     */
    public function checkBackupStatus(CpanelApiService $cpanel): array
    {
        $result = $cpanel->getBackupStatus();

        if (! ($result['success'] ?? false)) {
            throw new Exception($result['message'] ?? __('Failed to check backup status'));
        }

        return $result;
    }

    /**
     * Download a backup file from the cPanel server to a local path.
     *
     * @return string The local path where the backup was saved
     *
     * @throws Exception
     */
    public function downloadBackup(CpanelApiService $cpanel, string $filename, string $destPath, ?callable $progressCallback = null): string
    {
        $localPath = $destPath.'/'.basename($filename);

        $result = $cpanel->downloadFileToPath($filename, $localPath, $progressCallback);

        if (! ($result['success'] ?? false)) {
            throw new Exception($result['message'] ?? __('Failed to download backup'));
        }

        return $localPath;
    }

    /**
     * Analyze a cPanel backup archive via the agent.
     *
     * @return array<string, mixed> The discovered data from the backup
     *
     * @throws Exception
     */
    public function analyzeBackup(string $backupPath): array
    {
        $result = $this->agent->send('cpanel.analyze_backup', [
            'backup_path' => $backupPath,
        ]);

        if (! ($result['success'] ?? false)) {
            throw new Exception($result['error'] ?? __('Failed to analyze backup'));
        }

        return $result['data'] ?? [];
    }

    /**
     * Enqueue a restore job for a cPanel backup.
     *
     * @param  array{restoreFiles: bool, restoreDatabases: bool, restoreEmails: bool, restoreSsl: bool, discoveredData: array<string, mixed>}  $options
     * @return array{jobId: string, logPath: string}
     */
    public function enqueueRestore(User $user, string $backupPath, array $options): array
    {
        $jobId = (string) Str::uuid();
        $logDir = storage_path('app/migrations/cpanel');
        File::ensureDirectoryExists($logDir);
        $logPath = $logDir.'/'.$jobId.'.log';
        File::put($logPath, '');
        @chmod($logPath, 0644);

        $cacheKey = 'cpanel_restore_status_'.$jobId;
        Cache::put($cacheKey, ['status' => 'queued'], now()->addHours(2));

        RunCpanelRestore::dispatch(
            jobId: $jobId,
            logPath: $logPath,
            backupPath: $backupPath,
            username: $user->username,
            restoreFiles: $options['restoreFiles'] ?? true,
            restoreDatabases: $options['restoreDatabases'] ?? true,
            restoreEmails: $options['restoreEmails'] ?? true,
            restoreSsl: $options['restoreSsl'] ?? true,
            discoveredData: ! empty($options['discoveredData']) ? $options['discoveredData'] : null,
        );

        return [
            'jobId' => $jobId,
            'logPath' => $logPath,
        ];
    }

    /**
     * @param  array<string, mixed>  $domains
     */
    private function countDomains(array $domains): int
    {
        $count = 0;
        if (! empty($domains['main'])) {
            $count++;
        }
        $count += count($domains['addon'] ?? []);
        $count += count($domains['sub'] ?? []);

        return $count;
    }
}
