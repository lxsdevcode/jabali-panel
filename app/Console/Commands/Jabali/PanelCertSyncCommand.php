<?php

declare(strict_types=1);

namespace App\Console\Commands\Jabali;

use App\Models\PanelCertificate;
use Carbon\Carbon;
use Illuminate\Console\Command;

class PanelCertSyncCommand extends Command
{
    protected $signature = 'jabali:panel-cert-sync';

    protected $description = 'Sync panel SSL certificate info from the configured TLS cert file';

    public function handle(): int
    {
        $certPath = config('jabali.panel.tls_cert', '/etc/ssl/jabali/panel.crt');
        $hostname = config('jabali.panel.hostname') ?: config('app.url');

        // Extract hostname from APP_URL if needed
        $hostname = (string) preg_replace('#^https?://([^:/]+).*#', '$1', (string) $hostname);

        if (empty($hostname) || $hostname === 'localhost') {
            $this->warn('No panel hostname configured, skipping cert sync.');

            return self::SUCCESS;
        }

        $this->info("Syncing panel certificate for: {$hostname}");

        if (! file_exists($certPath)) {
            $this->warn("Certificate file not found: {$certPath}");

            $this->updateRecord($hostname, [
                'status' => 'pending',
                'last_renewal_at' => now(),
                'last_renewal_result' => 'not_found',
                'last_error' => "Certificate file not found: {$certPath}",
            ]);

            return self::SUCCESS;
        }

        $certContent = file_get_contents($certPath);
        if (! $certContent) {
            $this->error("Failed to read certificate file: {$certPath}");

            return self::FAILURE;
        }

        $certInfo = openssl_x509_parse($certContent);
        if (! $certInfo) {
            $this->error('Failed to parse certificate');

            return self::FAILURE;
        }

        $issuer = $certInfo['issuer']['O'] ?? $certInfo['issuer']['CN'] ?? 'Unknown';
        $isSelfSigned = ($certInfo['issuer']['CN'] ?? '') === ($certInfo['subject']['CN'] ?? '');

        $this->updateRecord($hostname, [
            'status' => $isSelfSigned ? 'self_signed' : 'active',
            'issuer' => $issuer,
            'issued_at' => isset($certInfo['validFrom_time_t'])
                ? Carbon::createFromTimestamp($certInfo['validFrom_time_t'])
                : null,
            'expires_at' => isset($certInfo['validTo_time_t'])
                ? Carbon::createFromTimestamp($certInfo['validTo_time_t'])
                : null,
            'last_renewal_at' => now(),
            'last_renewal_result' => 'success',
            'last_error' => null,
            'serial_number' => $certInfo['serialNumberHex'] ?? null,
        ]);

        $this->info("Certificate synced: issuer={$issuer}, status=".($isSelfSigned ? 'self_signed' : 'active'));

        return self::SUCCESS;
    }

    private function updateRecord(string $hostname, array $data): void
    {
        PanelCertificate::updateOrCreate(
            ['hostname' => $hostname],
            $data,
        );
    }
}
