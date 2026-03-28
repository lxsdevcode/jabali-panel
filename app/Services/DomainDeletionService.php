<?php

declare(strict_types=1);

namespace App\Services;

use App\Models\Domain;
use App\Services\Agent\AgentClient;
use Illuminate\Support\Facades\Log;

class DomainDeletionService
{
    public function __construct(private AgentClient $agent) {}

    public function delete(Domain $domain): void
    {
        $this->beforeDelete($domain);
        $domain->delete();
    }

    public function beforeDelete(Domain $domain): void
    {
        $this->cleanupForwarders($domain);
        $this->cleanupRelatedRecords($domain);
    }

    private function cleanupForwarders(Domain $domain): void
    {
        try {
            $domain->loadMissing('user', 'emailDomain.forwarders');
            $username = $domain->user?->username;

            if ($username && $domain->emailDomain) {
                foreach ($domain->emailDomain->forwarders as $forwarder) {
                    $this->agent->send('email.forwarder_delete', [
                        'username' => $username,
                        'email' => $forwarder->email,
                    ]);
                }
            }
        } catch (\Throwable $e) {
            Log::warning("Failed to delete email forwarders for domain {$domain->domain}: ".$e->getMessage());
        }
    }

    private function cleanupRelatedRecords(Domain $domain): void
    {
        $domain->sslCertificates()->delete();
        $domain->dnsRecords()->delete();
        $domain->redirects()->delete();
        $domain->hotlinkSetting?->delete();
        $domain->aliases()->delete();

        if ($domain->emailDomain) {
            $domain->emailDomain->mailboxes()->delete();
            $domain->emailDomain->forwarders()->delete();
            $domain->emailDomain->delete();
        }
    }
}
