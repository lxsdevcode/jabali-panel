<?php

declare(strict_types=1);

namespace App\Services\Agent;

class DomainAgentService
{
    public function __construct(private AgentClient $agent) {}

    public function create(string $username, string $domain): array
    {
        return $this->agent->domainCreate($username, $domain);
    }

    public function delete(string $username, string $domain, bool $deleteFiles = false): array
    {
        return $this->agent->domainDelete($username, $domain, $deleteFiles);
    }

    public function list(string $username): array
    {
        return $this->agent->domainList($username);
    }

    public function toggle(string $username, string $domain, bool $enable): array
    {
        return $this->agent->domainToggle($username, $domain, $enable);
    }

    public function aliasAdd(string $username, string $domain, string $alias): array
    {
        return $this->agent->domainAliasAdd($username, $domain, $alias);
    }

    public function aliasRemove(string $username, string $domain, string $alias): array
    {
        return $this->agent->domainAliasRemove($username, $domain, $alias);
    }

    public function ensureErrorPages(string $username, string $domain): array
    {
        return $this->agent->domainEnsureErrorPages($username, $domain);
    }
}
