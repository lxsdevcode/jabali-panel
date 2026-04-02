<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class SslPanelCommand extends JabaliCommand
{
    protected $signature = 'jabali:ssl:panel {--json} {--yes}';

    protected $description = 'Show panel SSL certificate status';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $certPath = '/etc/ssl/jabali/panel.crt';
        $hostname = gethostname() ?: 'unknown';

        if (! file_exists($certPath)) {
            $this->formatter()->error('No panel certificate found at '.$certPath);

            return Command::FAILURE;
        }

        $certContent = file_get_contents($certPath);
        $certData = openssl_x509_parse($certContent);

        if (! $certData) {
            $this->formatter()->error('Failed to parse panel certificate');

            return Command::FAILURE;
        }

        $issuer = $certData['issuer']['O'] ?? $certData['issuer']['CN'] ?? 'Unknown';
        $subjectCN = $certData['subject']['CN'] ?? 'Unknown';
        $issuerCN = $certData['issuer']['CN'] ?? '';
        $isSelfSigned = ($issuerCN === $subjectCN) && ! str_contains(strtolower($issuerCN), 'encrypt');
        $validFrom = date('Y-m-d H:i', $certData['validFrom_time_t'] ?? 0);
        $validTo = date('Y-m-d H:i', $certData['validTo_time_t'] ?? 0);
        $expiryTime = $certData['validTo_time_t'] ?? 0;
        $daysRemaining = (int) floor(($expiryTime - time()) / 86400);
        $isExpired = $expiryTime < time();

        $sans = [];
        if (isset($certData['extensions']['subjectAltName'])) {
            preg_match_all('/DNS:([^,\s]+)/', $certData['extensions']['subjectAltName'], $m);
            $sans = $m[1] ?? [];
        }

        $data = [
            'hostname' => $hostname,
            'subject' => $subjectCN,
            'issuer' => $issuer,
            'type' => $isSelfSigned ? 'Self-signed' : 'Let\'s Encrypt',
            'valid_from' => $validFrom,
            'valid_to' => $validTo,
            'days_remaining' => $daysRemaining,
            'expired' => $isExpired,
            'sans' => $sans,
            'path' => $certPath,
        ];

        if ($this->option('json')) {
            $this->formatter()->json($data);

            return Command::SUCCESS;
        }

        $status = $isExpired ? '<fg=red>Expired</>' : ($isSelfSigned ? '<fg=yellow>Self-signed</>' : '<fg=green>Valid</>');
        $daysColored = $this->colorDays($daysRemaining);

        $this->formatter()->info("Hostname:  {$hostname}");
        $this->formatter()->info("Subject:   {$subjectCN}");
        $this->formatter()->info("Status:    {$status}");
        $this->formatter()->info("Issuer:    {$issuer}");
        $this->formatter()->info("From:      {$validFrom}");
        $this->formatter()->info("To:        {$validTo}");
        $this->formatter()->info("Days left: {$daysColored}");

        if (! empty($sans)) {
            $this->formatter()->info('SANs:      '.implode(', ', $sans));
        }

        if (! in_array($hostname, $sans, true) && $subjectCN !== $hostname) {
            $this->formatter()->warning("Certificate does NOT cover hostname {$hostname}!");
        }

        return Command::SUCCESS;
    }

    private function colorDays(int $days): string
    {
        if ($days > 30) {
            return "<fg=green>{$days}</>";
        }

        if ($days > 7) {
            return "<fg=yellow>{$days}</>";
        }

        return "<fg=red>{$days}</>";
    }
}
