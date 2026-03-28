<?php

declare(strict_types=1);

namespace App\Services\Agent;

class EmailAgentService
{
    public function __construct(private AgentClient $agent) {}

    // Domain-level email operations

    public function enableDomain(string $username, string $domain): array
    {
        return $this->agent->emailEnableDomain($username, $domain);
    }

    public function disableDomain(string $username, string $domain): array
    {
        return $this->agent->emailDisableDomain($username, $domain);
    }

    public function getDomainInfo(string $username, string $domain): array
    {
        return $this->agent->emailGetDomainInfo($username, $domain);
    }

    public function generateDkim(string $username, string $domain, string $selector = 'default'): array
    {
        return $this->agent->emailGenerateDkim($username, $domain, $selector);
    }

    public function syncMaps(array $domains, array $mailboxes, array $aliases): array
    {
        return $this->agent->emailSyncMaps($domains, $mailboxes, $aliases);
    }

    public function syncVirtualUsers(string $domain): array
    {
        return $this->agent->emailSyncVirtualUsers($domain);
    }

    public function reloadServices(): array
    {
        return $this->agent->emailReloadServices();
    }

    // ACL operations

    public function aclShareFolder(string $username, string $ownerEmail, string $sharedWithEmail, string $folder, string $aclRights): array
    {
        return $this->agent->emailAclShareFolder($username, $ownerEmail, $sharedWithEmail, $folder, $aclRights);
    }

    public function aclUpdateFolder(string $username, string $ownerEmail, string $sharedWithEmail, string $folder, string $aclRights): array
    {
        return $this->agent->emailAclUpdateFolder($username, $ownerEmail, $sharedWithEmail, $folder, $aclRights);
    }

    public function aclRevokeFolder(string $username, string $ownerEmail, string $sharedWithEmail, string $folder): array
    {
        return $this->agent->emailAclRevokeFolder($username, $ownerEmail, $sharedWithEmail, $folder);
    }

    // Mailbox operations

    public function mailboxCreate(string $username, string $email, string $password, int $quotaBytes = 1073741824): array
    {
        return $this->agent->mailboxCreate($username, $email, $password, $quotaBytes);
    }

    public function mailboxDelete(string $username, string $email, bool $deleteFiles = false, ?string $maildirPath = null): array
    {
        return $this->agent->mailboxDelete($username, $email, $deleteFiles, $maildirPath);
    }

    public function mailboxToggle(string $username, string $email, bool $active): array
    {
        return $this->agent->mailboxToggle($username, $email, $active);
    }

    public function mailboxChangePassword(string $username, string $email, string $password): array
    {
        return $this->agent->mailboxChangePassword($username, $email, $password);
    }

    public function mailboxSetQuota(string $username, string $email, int $quotaBytes): array
    {
        return $this->agent->mailboxSetQuota($username, $email, $quotaBytes);
    }

    public function mailboxGetQuotaUsage(string $username, string $email): array
    {
        return $this->agent->mailboxGetQuotaUsage($username, $email);
    }

    // Mail queue operations

    public function queueList(): array
    {
        return $this->agent->mailQueueList();
    }

    public function queueRetry(string $id): array
    {
        return $this->agent->mailQueueRetry($id);
    }

    public function queueDelete(string $id): array
    {
        return $this->agent->mailQueueDelete($id);
    }

    // Spam settings

    public function rspamdUserSettings(string $username, array $whitelist = [], array $blacklist = [], ?float $score = null): array
    {
        return $this->agent->rspamdUserSettings($username, $whitelist, $blacklist, $score);
    }
}
