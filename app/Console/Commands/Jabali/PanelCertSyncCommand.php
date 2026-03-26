<?php

declare(strict_types=1);

namespace App\Console\Commands\Jabali;

use App\Models\PanelCertificate;
use Illuminate\Console\Command;

class PanelCertSyncCommand extends Command
{
    protected $signature = 'jabali:panel-cert-sync';

    protected $description = 'Sync panel SSL certificate info from FrankenPHP/Caddy storage';

    public function handle(): int
    {
        $certStorage = config('jabali.panel.cert_storage', '/var/lib/jabali/caddy');
        $hostname = config('jabali.panel.hostname') ?: config('app.url');

        // Extract hostname from APP_URL if needed
        $hostname = (string) preg_replace('#^https?://([^:/]+).*#', '$1', (string) $hostname);

        if (empty($hostname) || $hostname === 'localhost') {
            $this->warn('No panel hostname configured, skipping cert sync.');

            return self::SUCCESS;
        }

        $this->info("Syncing panel certificate for: {$hostname}");

        // Caddy stores certs in: {storage}/certificates/{issuer_key}/{hostname}/{hostname}.crt
        $certPath = $this->findCertificate($certStorage, $hostname);

        if (! $certPath) {
            $this->warn("No certificate found for {$hostname} in {$certStorage}");

            $this->updateRecord($hostname, [
                'status' => 'pending',
                'last_renewal_at' => now(),
                'last_renewal_result' => 'not_found',
                'last_error' => "No certificate file found in {$certStorage}",
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
        $isSelfSigned = str_contains($issuer, 'Caddy Local Authority')
            || ($certInfo['issuer']['CN'] ?? '') === ($certInfo['subject']['CN'] ?? '');

        $this->updateRecord($hostname, [
            'status' => $isSelfSigned ? 'self_signed' : 'active',
            'issuer' => $issuer,
            'issued_at' => isset($certInfo['validFrom_time_t'])
                ? date('Y-m-d H:i:s', $certInfo['validFrom_time_t'])
                : null,
            'expires_at' => isset($certInfo['validTo_time_t'])
                ? date('Y-m-d H:i:s', $certInfo['validTo_time_t'])
                : null,
            'last_renewal_at' => now(),
            'last_renewal_result' => 'success',
            'last_error' => null,
            'serial_number' => $certInfo['serialNumberHex'] ?? null,
        ]);

        $this->info("Certificate synced: issuer={$issuer}, status=".($isSelfSigned ? 'self_signed' : 'active'));

        return self::SUCCESS;
    }

    private function findCertificate(string $storage, string $hostname): ?string
    {
        // Check common Caddy certificate paths
        $patterns = [
            "{$storage}/certificates/acme-v02.api.letsencrypt.org-directory/{$hostname}/{$hostname}.crt",
            "{$storage}/certificates/acme.zerossl.com-v2-DV90/{$hostname}/{$hostname}.crt",
            "{$storage}/certificates/local/{$hostname}/{$hostname}.crt",
        ];

        foreach ($patterns as $path) {
            if (file_exists($path)) {
                return $path;
            }
        }

        // Fallback: glob search
        $globPattern = "{$storage}/certificates/*/{$hostname}/{$hostname}.crt";
        $results = glob($globPattern);

        return $results[0] ?? null;
    }

    private function updateRecord(string $hostname, array $data): void
    {
        PanelCertificate::updateOrCreate(
            ['hostname' => $hostname],
            $data,
        );
    }
}
