<?php

declare(strict_types=1);

namespace App\Services\Migration;

use App\Models\User;
use App\Services\Agent\AgentClient;
use App\Support\Formatter;
use App\Support\ServerFacts;
use Exception;
use Illuminate\Support\Facades\Hash;
use Illuminate\Support\Facades\Log;

class WhmMigrationOrchestrator
{
    private bool $shouldReloadFpm = false;

    private bool $restoreFiles;

    private bool $restoreDatabases;

    private bool $restoreEmails;

    private bool $restoreSsl;

    private bool $createLinuxUsers;

    public function __construct(
        private AgentClient $agent,
        private MigrationDnsSyncService $dnsSyncService,
        private MigrationEmailProvisionService $emailProvisionService,
    ) {}

    public function run(
        string $cacheKey,
        string $hostname,
        string $whmUsername,
        string $apiToken,
        int $port,
        bool $useSSL,
        array $accounts,
        array $selectedAccounts,
        bool $restoreFiles,
        bool $restoreDatabases,
        bool $restoreEmails,
        bool $restoreSsl,
        bool $createLinuxUsers,
    ): void {
        $this->restoreFiles = $restoreFiles;
        $this->restoreDatabases = $restoreDatabases;
        $this->restoreEmails = $restoreEmails;
        $this->restoreSsl = $restoreSsl;
        $this->createLinuxUsers = $createLinuxUsers;

        $store = new WhmMigrationStatusStore($cacheKey);
        $store->initialize($selectedAccounts);
        $reloadDelaySeconds = 15;

        try {
            $whm = new WhmApiService(
                $hostname,
                $whmUsername,
                $apiToken,
                $port,
                $useSSL,
            );

            $accountsByUser = $this->indexAccountsByUser($accounts);

            foreach ($selectedAccounts as $cpanelUser) {
                $this->migrateAccount($store, $whm, $cpanelUser, $accountsByUser[$cpanelUser] ?? []);
            }

            $store->setMigrating(false);

            try {
                $this->agent->send('service.reload', ['service' => 'nginx']);
            } catch (Exception $e) {
                Log::warning('Failed to reload nginx after WHM migration', ['error' => $e->getMessage()]);
            }

            if ($this->shouldReloadFpm) {
                try {
                    $this->agent->send('php.reload_all_fpm', [
                        'background' => true,
                        'delay' => $reloadDelaySeconds,
                    ]);
                } catch (Exception $e) {
                    Log::warning('Failed to reload PHP-FPM after WHM migration', ['error' => $e->getMessage()]);
                }
            }
        } catch (Exception $e) {
            Log::error('WHM migration batch failed', ['error' => $e->getMessage()]);
        } finally {
            $state = $store->get();
            if (($state['isMigrating'] ?? false) === true) {
                $store->setMigrating(false);
            }
        }
    }

    /**
     * @param  array<string, mixed>  $account
     */
    private function migrateAccount(WhmMigrationStatusStore $store, WhmApiService $whm, string $cpanelUser, array $account): void
    {
        $store->updateAccountStatus($cpanelUser, 'processing', __('Starting migration...'));

        try {
            $domain = $account['domain'] ?? '';
            $email = $account['email'] ?? ($domain !== '' ? "{$cpanelUser}@{$domain}" : "{$cpanelUser}@example.com");

            $user = $this->createOrGetUser($cpanelUser, $email);
            if (! $user) {
                throw new Exception(__('Failed to create user'));
            }

            $store->addAccountLog($cpanelUser, __('User ready: :username', ['username' => $user->username]), 'success');

            $store->updateAccountStatus($cpanelUser, 'backup_creating', __('Setting up backup transfer...'));

            $destPath = $this->getBackupDestPath();

            if (! is_dir($destPath)) {
                mkdir($destPath, 0755, true);
            }

            $backupPath = null;

            // Try SCP transfer first (requires SSH key setup)
            try {
                $keyName = $this->getSshKeyName();

                $this->agent->send('jabali_ssh.ensure_exists', []);

                $publicKeyResult = $this->agent->send('jabali_ssh.get_public_key', []);
                if (! ($publicKeyResult['success'] ?? false) || ! ($publicKeyResult['exists'] ?? false)) {
                    throw new Exception(__('Failed to get Jabali public key'));
                }
                $publicKey = $publicKeyResult['public_key'] ?? null;

                $this->agent->send('jabali_ssh.add_to_authorized_keys', [
                    'public_key' => $publicKey,
                    'comment' => 'whm-migration-'.$cpanelUser,
                ]);

                $privateKeyResult = $this->agent->send('jabali_ssh.get_private_key', []);
                if (! ($privateKeyResult['success'] ?? false) || ! ($privateKeyResult['exists'] ?? false)) {
                    throw new Exception(__('Failed to read Jabali private key'));
                }

                $privateKey = $privateKeyResult['private_key'] ?? null;
                if (empty($privateKey)) {
                    throw new Exception(__('Private key is empty'));
                }

                $store->addAccountLog($cpanelUser, __('Importing SSH key to cPanel...'), 'pending');

                // Delete any existing key first to avoid private/public mismatch
                $whm->deleteSshKey($cpanelUser, $keyName);

                $importResult = $whm->importSshPrivateKey($cpanelUser, $keyName, $privateKey);
                if (! ($importResult['success'] ?? false)) {
                    throw new Exception($importResult['message'] ?? __('Failed to import SSH key'));
                }

                $actualKeyName = $importResult['actual_key_name'] ?? $keyName;

                // Import public key (cPanel needs both to authorize)
                if (! empty($publicKey)) {
                    $whm->importSshPublicKey($cpanelUser, $actualKeyName, $publicKey);
                }

                $store->addAccountLog($cpanelUser, __('SSH key imported'), 'success');

                $authResult = $whm->authorizeSshKey($cpanelUser, $actualKeyName);
                if (! ($authResult['success'] ?? false)) {
                    $store->addAccountLog($cpanelUser, __('SSH key authorization skipped'), 'info');
                } else {
                    $store->addAccountLog($cpanelUser, __('SSH key authorized'), 'success');
                }

                $store->addAccountLog($cpanelUser, __('Initiating backup transfer...'), 'pending');

                $jabaliIp = $this->getJabaliPublicIp();

                $backupResult = $whm->createBackupToScpWithKey(
                    $cpanelUser,
                    $jabaliIp,
                    'root',
                    $destPath,
                    $actualKeyName,
                    22
                );

                if ($backupResult['success'] ?? false) {
                    $store->addAccountLog($cpanelUser, __('Backup initiated, transferring via SCP...'), 'success');
                    $store->updateAccountStatus($cpanelUser, 'backup_downloading', __('Waiting for backup file...'));
                    $backupPath = $this->waitForBackupFile($store, $cpanelUser, $destPath);
                }
            } catch (Exception $e) {
                Log::warning('SCP backup transfer failed, will try HTTP download', [
                    'user' => $cpanelUser,
                    'error' => $e->getMessage(),
                ]);
            }

            // Fallback to HTTP download if SCP failed or file didn't arrive
            if (! $backupPath) {
                $store->addAccountLog($cpanelUser, __('SCP transfer unavailable, downloading backup via HTTP...'), 'info');
                $backupPath = $this->downloadBackupViaHttp($whm, $store, $cpanelUser, $destPath);
            }

            if (! $backupPath) {
                throw new Exception(__('Failed to obtain backup file via SCP or HTTP download'));
            }

            $store->addAccountLog($cpanelUser, __('Backup received: :size', ['size' => Formatter::bytes(filesize($backupPath))]), 'success');

            $summary = $whm->getUserMigrationSummary($cpanelUser);
            $discoveredData = $whm->convertApiDataToAgentFormat($summary);

            $store->updateAccountStatus($cpanelUser, 'restoring', __('Restoring data...'));

            $result = $this->agent->send('cpanel.restore_backup', [
                'backup_path' => $backupPath,
                'username' => $user->username,
                'restore_files' => $this->restoreFiles,
                'restore_databases' => $this->restoreDatabases,
                'restore_emails' => $this->restoreEmails,
                'restore_ssl' => $this->restoreSsl,
                'discovered_data' => $discoveredData,
            ]);

            if ($result['success'] ?? false) {
                foreach ($result['log'] ?? [] as $entry) {
                    $store->addAccountLog($cpanelUser, $entry['message'], $entry['status'] ?? 'info');
                }

                $this->syncDnsZones($user, $discoveredData);
                $this->provisionEmails($store, $user, $cpanelUser, $discoveredData);

                $store->updateAccountStatus($cpanelUser, 'completed', __('Migration completed'));
                @unlink($backupPath);
            } else {
                throw new Exception($result['error'] ?? __('Restore failed'));
            }
        } catch (Exception $e) {
            Log::error('WHM migration failed for user', ['user' => $cpanelUser, 'error' => $e->getMessage()]);
            $store->updateAccountStatus($cpanelUser, 'error', $e->getMessage(), 'error');
        }
    }

    private function createOrGetUser(string $cpanelUser, string $email): ?User
    {
        $existingUser = User::where('username', $cpanelUser)->first();
        if ($existingUser) {
            return $existingUser;
        }

        if (User::where('email', $email)->exists()) {
            $parts = explode('@', $email);
            $emailDomain = $parts[1] ?? 'localhost';
            $email = "{$cpanelUser}.".time()."@{$emailDomain}";
        }

        $password = bin2hex(random_bytes(12));

        try {
            if ($this->createLinuxUsers) {
                $linuxUserExists = false;
                if (function_exists('posix_getpwnam')) {
                    $linuxUserExists = posix_getpwnam($cpanelUser) !== false;
                } else {
                    $linuxUserExists = is_dir('/home/'.$cpanelUser);
                }

                if (! $linuxUserExists) {
                    $result = $this->agent->send('user.create', [
                        'username' => $cpanelUser,
                        'password' => $password,
                    ]);

                    if (! ($result['success'] ?? false)) {
                        throw new Exception($result['error'] ?? __('Failed to create system user'));
                    }

                    if (($result['fpm_pool_created'] ?? false) === true) {
                        $this->shouldReloadFpm = true;
                    }
                }
            }

            return User::create([
                'name' => ucfirst($cpanelUser),
                'username' => $cpanelUser,
                'email' => $email,
                'password' => Hash::make($password),
                'home_directory' => '/home/'.$cpanelUser,
                'disk_quota_mb' => null,
                'is_active' => true,
                'is_admin' => false,
            ]);
        } catch (Exception $e) {
            Log::error('Failed to create user', ['username' => $cpanelUser, 'error' => $e->getMessage()]);

            return null;
        }
    }

    private function waitForBackupFile(WhmMigrationStatusStore $store, string $cpanelUser, string $destPath): ?string
    {
        $maxAttempts = 120;
        $attempt = 0;
        $lastSeenSize = 0;
        $sizeStableCount = 0;

        while ($attempt < $maxAttempts) {
            $attempt++;
            sleep(5);

            $pattern = "{$destPath}/backup-*_{$cpanelUser}.tar.gz";
            $files = glob($pattern);

            if (empty($files)) {
                $pattern = "{$destPath}/cpmove-{$cpanelUser}.tar.gz";
                $files = glob($pattern);
            }

            if (empty($files)) {
                if ($attempt % 6 === 0) {
                    $store->addAccountLog($cpanelUser, __('Waiting for backup file... (:count s)', ['count' => $attempt * 5]), 'pending');
                }

                continue;
            }

            usort($files, fn ($a, $b) => filemtime($b) - filemtime($a));
            $backupFile = $files[0];
            $currentSize = filesize($backupFile);

            if ($currentSize > 0 && $currentSize === $lastSeenSize) {
                $sizeStableCount++;
            } else {
                $sizeStableCount = 0;
            }
            $lastSeenSize = $currentSize;

            if ($sizeStableCount >= 3 && $currentSize >= 10 * 1024) {
                $this->agent->send('file.chown', [
                    'path' => $backupFile,
                    'owner' => 'www-data',
                    'group' => 'www-data',
                ]);

                $handle = fopen($backupFile, 'rb');
                $magic = $handle ? fread($handle, 2) : '';
                if ($handle) {
                    fclose($handle);
                }

                if ($magic === "\x1f\x8b") {
                    return $backupFile;
                }

                $store->addAccountLog($cpanelUser, __('Invalid backup file format, waiting...'), 'warning');
                $sizeStableCount = 0;
            }

            if ($attempt % 6 === 0) {
                $store->addAccountLog($cpanelUser, __('Receiving backup... :size', [
                    'size' => Formatter::bytes($currentSize),
                ]), 'pending');
            }
        }

        return null;
    }

    /**
     * Download a cPanel backup via HTTP when SCP transfer is unavailable.
     *
     * Creates a backup in the user's homedir, polls until complete, then
     * downloads the file via a cPanel session.
     *
     * @return string|null Local path to the downloaded backup file, or null on failure
     */
    private function downloadBackupViaHttp(WhmApiService $whm, WhmMigrationStatusStore $store, string $cpanelUser, string $destPath): ?string
    {
        try {
            // Snapshot existing backups so we can detect the new one
            $existingBackups = [];
            $listResult = $whm->listBackupsForUser($cpanelUser);
            if ($listResult['success'] ?? false) {
                foreach ($listResult['backups'] ?? [] as $backup) {
                    $existingBackups[$backup['file']] = true;
                }
            }

            $store->updateAccountStatus($cpanelUser, 'backup_creating', __('Creating backup in cPanel homedir...'));

            $createResult = $whm->createBackupForUser($cpanelUser);
            if (! ($createResult['success'] ?? false)) {
                Log::error('HTTP fallback: failed to create backup in homedir', [
                    'user' => $cpanelUser,
                    'message' => $createResult['message'] ?? 'Unknown error',
                ]);
                $store->addAccountLog($cpanelUser, __('Failed to create backup in homedir: :error', [
                    'error' => $createResult['message'] ?? __('Unknown error'),
                ]), 'error');

                return null;
            }

            $store->addAccountLog($cpanelUser, __('Backup creation started in cPanel homedir'), 'success');

            // Poll for the new backup to reach 'complete' status
            $maxAttempts = 120;
            $pollInterval = 10;
            $remotePath = null;

            for ($attempt = 1; $attempt <= $maxAttempts; $attempt++) {
                sleep($pollInterval);

                $listResult = $whm->listBackupsForUser($cpanelUser);
                if (! ($listResult['success'] ?? false)) {
                    if ($attempt % 6 === 0) {
                        $store->addAccountLog($cpanelUser, __('Waiting for backup to complete... (:time s)', [
                            'time' => $attempt * $pollInterval,
                        ]), 'pending');
                    }

                    continue;
                }

                foreach ($listResult['backups'] ?? [] as $backup) {
                    $file = $backup['file'] ?? '';
                    if (isset($existingBackups[$file])) {
                        continue;
                    }

                    if (($backup['status'] ?? '') === 'complete') {
                        $remotePath = $backup['path'] ?? null;
                        break 2;
                    }
                }

                // Log progress every 60 seconds
                if ($attempt % 6 === 0) {
                    $store->addAccountLog($cpanelUser, __('Waiting for backup to complete... (:time s)', [
                        'time' => $attempt * $pollInterval,
                    ]), 'pending');
                }
            }

            if (! $remotePath) {
                Log::error('HTTP fallback: backup did not complete in time', ['user' => $cpanelUser]);
                $store->addAccountLog($cpanelUser, __('Backup did not complete in time'), 'error');

                return null;
            }

            $store->addAccountLog($cpanelUser, __('Backup ready, downloading via HTTP...'), 'success');
            $store->updateAccountStatus($cpanelUser, 'backup_downloading', __('Downloading backup via HTTP...'));

            $localPath = "{$destPath}/backup-{$cpanelUser}.tar.gz";

            $downloadResult = $whm->downloadFileFromUser($cpanelUser, $remotePath, $localPath);
            if (! ($downloadResult['success'] ?? false)) {
                Log::error('HTTP fallback: download failed', [
                    'user' => $cpanelUser,
                    'message' => $downloadResult['message'] ?? 'Unknown error',
                ]);
                $store->addAccountLog($cpanelUser, __('HTTP download failed: :error', [
                    'error' => $downloadResult['message'] ?? __('Unknown error'),
                ]), 'error');

                return null;
            }

            // Validate the downloaded file is a valid gzip archive
            $handle = fopen($localPath, 'rb');
            $magic = $handle ? fread($handle, 2) : '';
            if ($handle) {
                fclose($handle);
            }

            if ($magic !== "\x1f\x8b") {
                Log::error('HTTP fallback: downloaded file is not a valid gzip archive', ['user' => $cpanelUser]);
                $store->addAccountLog($cpanelUser, __('Downloaded file is not a valid backup archive'), 'error');
                @unlink($localPath);

                return null;
            }

            $store->addAccountLog($cpanelUser, __('Backup downloaded via HTTP: :size', [
                'size' => Formatter::bytes($downloadResult['size'] ?? filesize($localPath)),
            ]), 'success');

            return $localPath;
        } catch (Exception $e) {
            Log::error('HTTP fallback: unexpected error', [
                'user' => $cpanelUser,
                'error' => $e->getMessage(),
            ]);
            $store->addAccountLog($cpanelUser, __('HTTP download failed: :error', [
                'error' => $e->getMessage(),
            ]), 'error');

            return null;
        }
    }

    /**
     * @param  array<int, array<string, mixed>>  $accounts
     * @return array<string, array<string, mixed>>
     */
    private function indexAccountsByUser(array $accounts): array
    {
        $indexed = [];

        foreach ($accounts as $account) {
            if (! isset($account['user'])) {
                continue;
            }

            $indexed[$account['user']] = $account;
        }

        return $indexed;
    }

    private function getBackupDestPath(): string
    {
        return '/var/backups/jabali/whm-migrations';
    }

    private function getSshKeyName(): string
    {
        return 'jabali-system-key';
    }

    /**
     * @param  array<string, mixed>  $discoveredData
     */
    private function provisionEmails(WhmMigrationStatusStore $store, User $user, string $cpanelUser, array $discoveredData): void
    {
        if (! $this->restoreEmails) {
            return;
        }

        $mailboxes = $discoveredData['mailboxes'] ?? [];
        $forwarders = $discoveredData['forwarders'] ?? [];

        if (empty($mailboxes) && empty($forwarders)) {
            return;
        }

        try {
            $store->addAccountLog($cpanelUser, __('Provisioning email accounts...'), 'pending');
            $result = $this->emailProvisionService->provisionFromDiscoveredData(
                $user,
                $discoveredData,
                fn (string $msg, string $status) => $store->addAccountLog($cpanelUser, $msg, $status),
            );

            $summary = __(':mb mailbox(es), :fw forwarder(s)', [
                'mb' => count($result->mailboxes),
                'fw' => count($result->forwarders),
            ]);

            if (! empty($result->errors)) {
                $summary .= ', '.__(':err error(s)', ['err' => count($result->errors)]);
            }

            $store->addAccountLog($cpanelUser, __('Email provisioning complete: :summary', ['summary' => $summary]), empty($result->errors) ? 'success' : 'warning');
        } catch (Exception $e) {
            Log::warning('Failed to provision emails after WHM migration', [
                'user' => $user->username,
                'error' => $e->getMessage(),
            ]);
            $store->addAccountLog($cpanelUser, __('Email provisioning failed: :error', ['error' => $e->getMessage()]), 'warning');
        }
    }

    /**
     * @param  array<string, mixed>  $discoveredData
     */
    private function syncDnsZones(User $user, array $discoveredData): void
    {
        try {
            $domains = $discoveredData['domains'] ?? [];
            $this->dnsSyncService->syncDomainsForUser($user, $domains);
        } catch (Exception $e) {
            Log::warning('Failed to sync DNS zones after WHM migration', [
                'user' => $user->username,
                'error' => $e->getMessage(),
            ]);
        }
    }

    private function getJabaliPublicIp(): string
    {
        $ip = ServerFacts::serverIp('');
        if ($ip !== '') {
            return $ip;
        }

        $fallback = gethostbyname(gethostname() ?: 'localhost');

        return is_string($fallback) ? $fallback : '';
    }
}
