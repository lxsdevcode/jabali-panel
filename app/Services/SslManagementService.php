<?php

declare(strict_types=1);

namespace App\Services;

use App\Models\Domain;
use App\Models\SslCertificate;
use App\Services\Agent\AgentClient;
use Carbon\Carbon;

class SslManagementService
{
    public function __construct(private AgentClient $agent) {}

    public function issue(Domain $domain, string $service = 'web'): SslCertificate
    {
        try {
            $result = $this->agent->sslIssue(
                $domain->domain,
                $domain->user->username,
                $domain->user->email,
                true,
            );

            $certData = [
                'type' => 'lets_encrypt',
                'status' => 'active',
                'issuer' => "Let's Encrypt",
                'certificate' => $result['certificate'] ?? null,
                'issued_at' => now(),
                'expires_at' => isset($result['valid_to']) ? Carbon::parse($result['valid_to']) : now()->addDays(90),
                'last_check_at' => now(),
                'last_error' => null,
                'renewal_attempts' => 0,
                'auto_renew' => true,
            ];

            $cert = SslCertificate::updateOrCreate(
                ['domain_id' => $domain->id, 'service' => $service],
                $certData,
            );

            // If this is a web cert that also covers mail.$domain, create the mail record too
            if ($service === 'web') {
                SslCertificate::updateOrCreate(
                    ['domain_id' => $domain->id, 'service' => 'mail'],
                    $certData,
                );
            }

            return $cert;
        } catch (\Exception $e) {
            return SslCertificate::updateOrCreate(
                ['domain_id' => $domain->id, 'service' => $service],
                [
                    'type' => 'lets_encrypt',
                    'status' => 'failed',
                    'last_check_at' => now(),
                    'last_error' => $e->getMessage(),
                ],
            );
        }
    }

    public function renew(Domain $domain, string $service = 'web'): SslCertificate
    {
        $cert = SslCertificate::where('domain_id', $domain->id)
            ->where('service', $service)
            ->firstOrFail();

        try {
            $result = $this->agent->sslRenew(
                $domain->domain,
                $domain->user->username,
            );

            $cert->update([
                'status' => 'active',
                'issued_at' => now(),
                'expires_at' => isset($result['valid_to']) ? Carbon::parse($result['valid_to']) : now()->addDays(90),
                'last_check_at' => now(),
                'last_error' => null,
                'renewal_attempts' => 0,
            ]);
        } catch (\Exception $e) {
            $cert->increment('renewal_attempts');
            $cert->update([
                'status' => 'failed',
                'last_check_at' => now(),
                'last_error' => $e->getMessage(),
            ]);
        }

        return $cert->fresh();
    }

    public function check(Domain $domain, string $service = 'web'): SslCertificate
    {
        $result = $this->agent->sslCheck(
            $domain->domain,
            $domain->user->username,
        );

        $ssl = $result['ssl'] ?? $result;
        $hasSsl = $ssl['has_ssl'] ?? false;

        if ($hasSsl) {
            $type = $ssl['type'] ?? 'none';
            if (! in_array($type, ['lets_encrypt', 'self_signed', 'custom'], true)) {
                // snakeoil or unknown certs — check if self-signed
                $type = ($ssl['is_self_signed'] ?? false) ? 'self_signed' : 'none';
            }

            return SslCertificate::updateOrCreate(
                ['domain_id' => $domain->id, 'service' => $service],
                [
                    'type' => $type,
                    'status' => ($ssl['is_expired'] ?? false) ? 'expired' : 'active',
                    'issuer' => $ssl['issuer'] ?? null,
                    'issued_at' => isset($ssl['valid_from']) ? Carbon::parse($ssl['valid_from']) : null,
                    'expires_at' => isset($ssl['valid_to']) ? Carbon::parse($ssl['valid_to']) : null,
                    'last_check_at' => now(),
                    'last_error' => null,
                ],
            );
        }

        return SslCertificate::updateOrCreate(
            ['domain_id' => $domain->id, 'service' => $service],
            [
                'status' => 'pending',
                'last_check_at' => now(),
            ],
        );
    }

    public function installCustom(Domain $domain, string $certificate, string $privateKey, ?string $caBundle = null): SslCertificate
    {
        $this->agent->sslInstall(
            $domain->domain,
            $domain->user->username,
            $certificate,
            $privateKey,
            $caBundle,
        );

        return SslCertificate::updateOrCreate(
            ['domain_id' => $domain->id, 'service' => 'web'],
            [
                'type' => 'custom',
                'status' => 'active',
                'certificate' => $certificate,
                'private_key' => $privateKey,
                'ca_bundle' => $caBundle,
                'issued_at' => now(),
                'last_check_at' => now(),
                'last_error' => null,
                'auto_renew' => false,
            ],
        );
    }

    public function generateSelfSigned(Domain $domain, int $days = 365): SslCertificate
    {
        $this->agent->sslGenerateSelfSigned(
            $domain->domain,
            $domain->user->username,
            $days,
        );

        return SslCertificate::updateOrCreate(
            ['domain_id' => $domain->id, 'service' => 'web'],
            [
                'type' => 'self_signed',
                'status' => 'active',
                'issuer' => 'Self-Signed',
                'issued_at' => now(),
                'expires_at' => now()->addDays($days),
                'last_check_at' => now(),
                'last_error' => null,
                'auto_renew' => false,
            ],
        );
    }
}
