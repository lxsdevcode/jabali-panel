<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class SystemDiskCommand extends JabaliCommand
{
    protected $signature = 'jabali:system:disk {--json} {--yes}';

    protected $description = 'Show disk usage information';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $total = disk_total_space('/');
        $free = disk_free_space('/');

        if ($total === false || $free === false) {
            $this->formatter()->error('Unable to read disk information');

            return Command::FAILURE;
        }

        $used = $total - $free;
        $percent = $total > 0 ? round(($used / $total) * 100, 1) : 0;

        $info = [
            'mount' => '/',
            'total' => $this->formatBytes($total),
            'used' => $this->formatBytes($used),
            'free' => $this->formatBytes($free),
            'use_percent' => "{$percent}%",
        ];

        if ($this->option('json')) {
            $this->formatter()->json($info);

            return Command::SUCCESS;
        }

        $this->formatter()->table(
            ['Mount', 'Total', 'Used', 'Free', 'Use%'],
            [['/', $info['total'], $info['used'], $info['free'], $info['use_percent']]]
        );

        return Command::SUCCESS;
    }

    private function formatBytes(float $bytes): string
    {
        $units = ['B', 'KB', 'MB', 'GB', 'TB'];
        $index = 0;

        while ($bytes >= 1024 && $index < count($units) - 1) {
            $bytes /= 1024;
            $index++;
        }

        return round($bytes, 1).' '.$units[$index];
    }
}
