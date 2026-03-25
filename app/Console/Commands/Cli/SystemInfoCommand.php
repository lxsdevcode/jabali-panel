<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class SystemInfoCommand extends JabaliCommand
{
    protected $signature = 'jabali:system:info {--json} {--yes}';

    protected $description = 'Show system information';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $info = [
            'hostname' => gethostname() ?: 'unknown',
            'kernel' => php_uname('r'),
            'uptime' => $this->getUptime(),
            'cpu_model' => $this->getCpuModel(),
            'cpu_cores' => $this->getCpuCores(),
            'memory' => $this->getTotalMemory(),
        ];

        if ($this->option('json')) {
            $this->formatter()->json($info);

            return Command::SUCCESS;
        }

        $rows = [];
        foreach ($info as $field => $value) {
            $rows[] = [ucfirst(str_replace('_', ' ', $field)), $value];
        }

        $this->formatter()->table(['Field', 'Value'], $rows);

        return Command::SUCCESS;
    }

    private function getUptime(): string
    {
        if (! is_readable('/proc/uptime')) {
            return 'unknown';
        }

        $content = file_get_contents('/proc/uptime');
        if ($content === false) {
            return 'unknown';
        }

        $seconds = (int) explode(' ', trim($content))[0];
        $days = intdiv($seconds, 86400);
        $hours = intdiv($seconds % 86400, 3600);
        $minutes = intdiv($seconds % 3600, 60);

        return "{$days}d {$hours}h {$minutes}m";
    }

    private function getCpuModel(): string
    {
        if (! is_readable('/proc/cpuinfo')) {
            return 'unknown';
        }

        $content = file_get_contents('/proc/cpuinfo');
        if ($content === false) {
            return 'unknown';
        }

        if (preg_match('/model name\s*:\s*(.+)/i', $content, $matches)) {
            return trim($matches[1]);
        }

        return 'unknown';
    }

    private function getCpuCores(): string
    {
        if (! is_readable('/proc/cpuinfo')) {
            return 'unknown';
        }

        $content = file_get_contents('/proc/cpuinfo');
        if ($content === false) {
            return 'unknown';
        }

        $count = preg_match_all('/^processor\s*:/m', $content);

        return (string) ($count ?: 'unknown');
    }

    private function getTotalMemory(): string
    {
        if (! is_readable('/proc/meminfo')) {
            return 'unknown';
        }

        $content = file_get_contents('/proc/meminfo');
        if ($content === false) {
            return 'unknown';
        }

        if (preg_match('/MemTotal:\s*(\d+)\s*kB/i', $content, $matches)) {
            $mb = round((int) $matches[1] / 1024);

            return "{$mb} MB";
        }

        return 'unknown';
    }
}
