<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class ServiceCommand extends JabaliCommand
{
    protected $signature = 'jabali:service:list {--json} {--yes}';

    protected $description = 'List managed services and their status';

    private const SERVICES = [
        'nginx', 'mariadb', 'redis-server', 'postfix', 'dovecot',
        'rspamd', 'clamav-daemon', 'named', 'opendkim', 'fail2ban',
        'ssh', 'cron',
    ];

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $services = $this->discoverServices();
        $result = $this->callAgent('service.list', ['services' => $services]);

        if ($result === null) {
            return Command::FAILURE;
        }

        $data = $result->get('services', []);

        if ($this->option('json')) {
            $this->formatter()->json($data);

            return Command::SUCCESS;
        }

        $rows = [];
        foreach ($data as $name => $status) {
            $active = ($status['is_active'] ?? false);
            $enabled = ($status['is_enabled'] ?? false);
            $rows[] = [
                $name,
                $active ? '<fg=green>Running</>' : '<fg=red>Stopped</>',
                $enabled ? '<fg=green>Enabled</>' : '<fg=gray>Disabled</>',
            ];
        }

        $this->formatter()->table(['Service', 'Status', 'Boot'], $rows);

        return Command::SUCCESS;
    }

    private function discoverServices(): array
    {
        $services = self::SERVICES;

        // Discover installed PHP-FPM versions from systemd
        $phpServices = glob('/lib/systemd/system/php*-fpm.service') ?: [];
        foreach ($phpServices as $svc) {
            if (preg_match('/php[\d.]+-fpm/', basename($svc, '.service'), $m)) {
                $services[] = $m[0];
            }
        }

        return $services;
    }
}
