<?php

declare(strict_types=1);

namespace App\Services;

use App\Models\Domain;
use App\Support\ServerFacts;
use Carbon\Carbon;
use Symfony\Component\Process\Process;

class DomainHealthService
{
    /**
     * @return array{status: string, resolved_ip: string|null, server_ip: string}
     */
    public function checkDns(Domain $domain): array
    {
        $serverIp = ServerFacts::serverIp('127.0.0.1');

        try {
            // Use dns_get_record for external DNS lookup (bypasses /etc/hosts)
            $records = dns_get_record($domain->domain, DNS_A);
            $resolvedIp = $records[0]['ip'] ?? null;

            if (! $resolvedIp) {
                return ['status' => 'dns_missing', 'resolved_ip' => null, 'server_ip' => $serverIp];
            }

            if ($resolvedIp === $serverIp) {
                return ['status' => 'points_here', 'resolved_ip' => $resolvedIp, 'server_ip' => $serverIp];
            }

            if ($this->isCloudflareIp($resolvedIp)) {
                return ['status' => 'cloudflare', 'resolved_ip' => $resolvedIp, 'server_ip' => $serverIp];
            }

            return ['status' => 'external', 'resolved_ip' => $resolvedIp, 'server_ip' => $serverIp];
        } catch (\Throwable $e) {
            return ['status' => 'dns_error', 'resolved_ip' => null, 'server_ip' => $serverIp];
        }
    }

    /**
     * Extract root domain from a hostname (e.g., mx.jabali-panel.com -> jabali-panel.com).
     */
    private function getRootDomain(string $domain): string
    {
        $parts = explode('.', $domain);

        if (count($parts) <= 2) {
            return $domain;
        }

        // Handle known two-part TLDs (co.uk, com.tr, com.br, etc.)
        $twoPartTlds = ['co.uk', 'co.il', 'com.tr', 'com.br', 'com.au', 'co.nz', 'co.za', 'com.mx', 'org.uk', 'net.au', 'org.au'];
        $lastTwo = implode('.', array_slice($parts, -2));

        if (in_array($lastTwo, $twoPartTlds, true)) {
            return implode('.', array_slice($parts, -3));
        }

        return implode('.', array_slice($parts, -2));
    }

    /**
     * @return array{status: string, expiry: string|null, registrar: string|null, raw: string|null}
     */
    public function checkWhois(Domain $domain): array
    {
        try {
            $whoisDomain = $this->getRootDomain($domain->domain);
            $process = new Process(['whois', $whoisDomain]);
            $process->setTimeout(10);
            $process->run();

            $output = $process->getOutput();

            // whois may exit non-zero but still return useful data on stdout
            if (empty(trim($output))) {
                return ['status' => 'whois_error', 'expiry' => null, 'registrar' => null, 'raw' => $process->getErrorOutput()];
            }

            // Check for "not found" / unregistered indicators
            if (preg_match('/No match|NOT FOUND|No Data Found|No entries found/i', $output)) {
                return ['status' => 'unregistered', 'expiry' => null, 'registrar' => null, 'raw' => $output];
            }

            // Parse expiry date
            $expiry = null;
            if (preg_match('/Expir.*?:\s*(.+)/im', $output, $m)) {
                try {
                    $expiry = Carbon::parse(trim($m[1]))->format('Y-m-d');
                } catch (\Throwable) {
                }
            } elseif (preg_match('/paid-till:\s*(.+)/im', $output, $m)) {
                try {
                    $expiry = Carbon::parse(trim($m[1]))->format('Y-m-d');
                } catch (\Throwable) {
                }
            }

            // Parse registrar
            $registrar = null;
            if (preg_match('/Registrar:\s*(.+)/im', $output, $m)) {
                $registrar = trim($m[1]);
            }

            // Check if expired
            if ($expiry && Carbon::parse($expiry)->isPast()) {
                return ['status' => 'expired', 'expiry' => $expiry, 'registrar' => $registrar, 'raw' => $output];
            }

            return ['status' => 'registered', 'expiry' => $expiry, 'registrar' => $registrar, 'raw' => $output];
        } catch (\Throwable $e) {
            return ['status' => 'whois_error', 'expiry' => null, 'registrar' => null, 'raw' => null];
        }
    }

    private function isCloudflareIp(string $ip): bool
    {
        if (filter_var($ip, FILTER_VALIDATE_IP, FILTER_FLAG_IPV4)) {
            return $this->ipInRanges($ip, config('cloudflare.ip_ranges.v4', []));
        }

        if (filter_var($ip, FILTER_VALIDATE_IP, FILTER_FLAG_IPV6)) {
            return $this->ipv6InRanges($ip, config('cloudflare.ip_ranges.v6', []));
        }

        return false;
    }

    private function ipInRanges(string $ip, array $ranges): bool
    {
        $ipLong = ip2long($ip);

        foreach ($ranges as $range) {
            [$subnet, $bits] = explode('/', $range);
            $subnetLong = ip2long($subnet);
            $mask = -1 << (32 - (int) $bits);
            if (($ipLong & $mask) === ($subnetLong & $mask)) {
                return true;
            }
        }

        return false;
    }

    private function ipv6InRanges(string $ip, array $ranges): bool
    {
        $ipBin = inet_pton($ip);
        if ($ipBin === false) {
            return false;
        }

        foreach ($ranges as $range) {
            [$subnet, $bits] = explode('/', $range);
            $subnetBin = inet_pton($subnet);
            if ($subnetBin === false) {
                continue;
            }
            $mask = str_repeat("\xff", (int) ($bits / 8));
            if ($bits % 8) {
                $mask .= chr(0xFF << (8 - ($bits % 8)));
            }
            $mask = str_pad($mask, 16, "\x00");
            if (($ipBin & $mask) === ($subnetBin & $mask)) {
                return true;
            }
        }

        return false;
    }
}
