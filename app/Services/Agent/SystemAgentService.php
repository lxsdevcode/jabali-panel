<?php

declare(strict_types=1);

namespace App\Services\Agent;

class SystemAgentService
{
    public function __construct(private AgentClient $agent) {}

    // User operations

    public function createUser(string $username, ?string $password = null): array
    {
        return $this->agent->createUser($username, $password);
    }

    public function deleteUser(string $username, bool $removeHome = false, array $domains = []): array
    {
        return $this->agent->deleteUser($username, $removeHome, $domains);
    }

    public function userExists(string $username): bool
    {
        return $this->agent->userExists($username);
    }

    // Cron operations

    public function cronCreate(string $username, string $schedule, string $command, string $comment = ''): array
    {
        return $this->agent->cronCreate($username, $schedule, $command, $comment);
    }

    public function cronDelete(string $username, string $command, string $schedule = ''): array
    {
        return $this->agent->cronDelete($username, $command, $schedule);
    }

    public function cronList(string $username): array
    {
        return $this->agent->cronList($username);
    }

    public function cronRun(string $username, string $command): array
    {
        return $this->agent->cronRun($username, $command);
    }

    public function cronToggle(string $username, string $command, bool $enable): array
    {
        return $this->agent->cronToggle($username, $command, $enable);
    }

    public function cronWordPressSetup(string $username, string $domain, string $schedule = '*/5 * * * *', bool $disable = false): array
    {
        return $this->agent->cronWordPressSetup($username, $domain, $schedule, $disable);
    }

    // Quota operations

    public function quotaEnable(string $mount = '/home'): array
    {
        return $this->agent->quotaEnable($mount);
    }

    public function quotaGet(string $username, string $mount = '/home'): array
    {
        return $this->agent->quotaGet($username, $mount);
    }

    public function quotaSet(string $username, int $softMb, int $hardMb = 0, string $mount = '/home'): array
    {
        return $this->agent->quotaSet($username, $softMb, $hardMb, $mount);
    }

    public function quotaReport(string $mount = '/home'): array
    {
        return $this->agent->quotaReport($mount);
    }

    public function quotaStatus(string $mount = '/home'): array
    {
        return $this->agent->quotaStatus($mount);
    }

    // Metrics operations

    public function metricsOverview(): array
    {
        return $this->agent->metricsOverview();
    }

    public function metricsCpu(): array
    {
        return $this->agent->metricsCpu();
    }

    public function metricsMemory(): array
    {
        return $this->agent->metricsMemory();
    }

    public function metricsDisk(): array
    {
        return $this->agent->metricsDisk();
    }

    public function metricsNetwork(): array
    {
        return $this->agent->metricsNetwork();
    }

    public function metricsProcesses(int $limit = 15, string $sortBy = 'cpu'): array
    {
        return $this->agent->metricsProcesses($limit, $sortBy);
    }

    // Server config operations

    public function serverExportConfig(string $outputPath = '/tmp/jabali-config-export.tar.gz', array $options = []): array
    {
        return $this->agent->serverExportConfig($outputPath, $options);
    }

    public function serverImportConfig(string $archivePath, bool $importNginx = true, bool $importDns = true, bool $dryRun = false): array
    {
        return $this->agent->serverImportConfig($archivePath, $importNginx, $importDns, $dryRun);
    }

    public function serverVersions(): array
    {
        return $this->agent->serverVersions();
    }

    // Updates operations

    public function updatesList(bool $refresh = false): array
    {
        return $this->agent->updatesList($refresh);
    }

    public function updatesRun(): array
    {
        return $this->agent->updatesRun();
    }

    // IP operations

    public function ipList(): array
    {
        return $this->agent->ipList();
    }

    public function ipAdd(string $ip, int $cidr, string $interface): array
    {
        return $this->agent->ipAdd($ip, $cidr, $interface);
    }

    public function ipRemove(string $ip, int $cidr, string $interface): array
    {
        return $this->agent->ipRemove($ip, $cidr, $interface);
    }

    public function ipInfo(string $ip): array
    {
        return $this->agent->ipInfo($ip);
    }

    // Git operations

    public function gitGenerateKey(string $username): array
    {
        return $this->agent->gitGenerateKey($username);
    }

    public function gitDeploy(string $username, string $repoUrl, string $branch, string $deployPath, ?string $deployScript = null): array
    {
        return $this->agent->gitDeploy($username, $repoUrl, $branch, $deployPath, $deployScript);
    }

    // Import operations

    public function importDiscover(int $importId, string $sourceType, string $importMethod, ?string $backupPath, ?string $remoteHost, ?int $remotePort, ?string $remoteUser, ?string $remotePassword): array
    {
        return $this->agent->importDiscover($importId, $sourceType, $importMethod, $backupPath, $remoteHost, $remotePort, $remoteUser, $remotePassword);
    }

    public function importStart(int $importId): array
    {
        return $this->agent->importStart($importId);
    }

    // Scanner operations

    public function scannerInstall(string $tool): array
    {
        return $this->agent->scannerInstall($tool);
    }

    public function scannerUninstall(string $tool): array
    {
        return $this->agent->scannerUninstall($tool);
    }

    public function scannerStatus(?string $tool = null): array
    {
        return $this->agent->scannerStatus($tool);
    }

    public function scannerRunNikto(string $target): array
    {
        return $this->agent->scannerRunNikto($target);
    }

    public function scannerRunLynis(): array
    {
        return $this->agent->scannerRunLynis();
    }

    public function scannerStartLynis(): array
    {
        return $this->agent->scannerStartLynis();
    }

    public function scannerGetScanStatus(string $scanner = 'lynis'): array
    {
        return $this->agent->scannerGetScanStatus($scanner);
    }

    // Security status

    public function clamavStatusLight(): array
    {
        return $this->agent->clamavStatusLight();
    }

    public function fail2banStatusLight(): array
    {
        return $this->agent->fail2banStatusLight();
    }

    // WAF operations

    public function wafApplySettings(bool $enabled, string $paranoia, bool $auditLog, array $whitelistRules = []): array
    {
        return $this->agent->wafApplySettings($enabled, $paranoia, $auditLog, $whitelistRules);
    }

    public function wafAuditLogList(int $limit = 200): array
    {
        return $this->agent->wafAuditLogList($limit);
    }

    // Geo operations

    public function geoApplyRules(array $rules): array
    {
        return $this->agent->geoApplyRules($rules);
    }

    public function geoUpdateDatabase(string $accountId, string $licenseKey, string $editionIds = 'GeoLite2-Country'): array
    {
        return $this->agent->geoUpdateDatabase($accountId, $licenseKey, $editionIds);
    }

    public function geoUploadDatabase(string $edition, string $content): array
    {
        return $this->agent->geoUploadDatabase($edition, $content);
    }

    // WordPress operations

    public function wpInstall(string $username, string $domain, array $options): array
    {
        return $this->agent->wpInstall($username, $domain, $options);
    }

    public function wpDelete(string $username, string $siteId, bool $deleteFiles = true, bool $deleteDatabase = true): array
    {
        return $this->agent->wpDelete($username, $siteId, $deleteFiles, $deleteDatabase);
    }

    public function wpList(string $username): array
    {
        return $this->agent->wpList($username);
    }

    public function wpUpdate(string $username, string $siteId, string $type = 'all'): array
    {
        return $this->agent->wpUpdate($username, $siteId, $type);
    }

    public function wpScan(string $username): array
    {
        return $this->agent->wpScan($username);
    }

    public function wpImport(string $username, string $path, ?int $domainId = null): array
    {
        return $this->agent->wpImport($username, $path, $domainId);
    }

    public function wpAutoLogin(string $username, string $siteId): array
    {
        return $this->agent->wpAutoLogin($username, $siteId);
    }

    public function wpCreateStaging(string $username, string $siteId, string $subdomain = 'staging', ?string $targetDomain = null): array
    {
        return $this->agent->wpCreateStaging($username, $siteId, $subdomain, $targetDomain);
    }

    public function wpPushStaging(string $username, string $stagingSiteId): array
    {
        return $this->agent->wpPushStaging($username, $stagingSiteId);
    }

    public function wpCacheEnable(string $username, string $siteId): array
    {
        return $this->agent->wpCacheEnable($username, $siteId);
    }

    public function wpCacheDisable(string $username, string $siteId, bool $removePlugin = false, bool $resetData = false): array
    {
        return $this->agent->wpCacheDisable($username, $siteId, $removePlugin, $resetData);
    }

    public function wpCacheFlush(string $username, string $siteId): array
    {
        return $this->agent->wpCacheFlush($username, $siteId);
    }

    public function wpCacheStatus(string $username, string $siteId): array
    {
        return $this->agent->wpCacheStatus($username, $siteId);
    }

    public function wpPageCacheEnable(string $username, string $domain, ?string $siteId = null): array
    {
        return $this->agent->wpPageCacheEnable($username, $domain, $siteId);
    }

    public function wpPageCacheDisable(string $username, string $domain, ?string $siteId = null): array
    {
        return $this->agent->wpPageCacheDisable($username, $domain, $siteId);
    }

    public function wpPageCachePurge(string $domain, ?string $path = null): array
    {
        return $this->agent->wpPageCachePurge($domain, $path);
    }

    public function wpPageCacheStatus(string $username, string $domain, ?string $siteId = null): array
    {
        return $this->agent->wpPageCacheStatus($username, $domain, $siteId);
    }
}
