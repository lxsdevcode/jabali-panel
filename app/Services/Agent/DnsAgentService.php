<?php

declare(strict_types=1);

namespace App\Services\Agent;

class DnsAgentService
{
    public function __construct(private AgentClient $agent) {}

    public function createZone(string $domain, array $settings = []): array
    {
        return $this->agent->dnsCreateZone($domain, $settings);
    }

    public function deleteZone(string $domain): array
    {
        return $this->agent->dnsDeleteZone($domain);
    }

    public function syncZone(string $domain, array $records, array $settings = []): array
    {
        return $this->agent->dnsSyncZone($domain, $records, $settings);
    }

    public function reload(): array
    {
        return $this->agent->dnsReload();
    }

    public function enableDnssec(string $domain): array
    {
        return $this->agent->dnsEnableDnssec($domain);
    }

    public function disableDnssec(string $domain): array
    {
        return $this->agent->dnsDisableDnssec($domain);
    }

    public function getDnssecStatus(string $domain): array
    {
        return $this->agent->dnsGetDnssecStatus($domain);
    }

    public function getDsRecords(string $domain): array
    {
        return $this->agent->dnsGetDsRecords($domain);
    }
}
