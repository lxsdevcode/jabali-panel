<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class SystemMemoryCommand extends JabaliCommand
{
    protected $signature = 'jabali:system:memory {--json} {--yes}';

    protected $description = 'Show memory usage information';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        if (! is_readable('/proc/meminfo')) {
            $this->formatter()->error('Unable to read /proc/meminfo');

            return Command::FAILURE;
        }

        $content = file_get_contents('/proc/meminfo');
        if ($content === false) {
            $this->formatter()->error('Unable to read /proc/meminfo');

            return Command::FAILURE;
        }

        $meminfo = $this->parseMeminfo($content);

        if ($this->option('json')) {
            $this->formatter()->json($meminfo);

            return Command::SUCCESS;
        }

        $rows = [];
        foreach ($meminfo as $entry) {
            $rows[] = [$entry['type'], $entry['total'], $entry['used'], $entry['free']];
        }

        $this->formatter()->table(['Type', 'Total', 'Used', 'Free'], $rows);

        return Command::SUCCESS;
    }

    private function parseMeminfo(string $content): array
    {
        $values = [];
        if (preg_match('/MemTotal:\s*(\d+)/i', $content, $m)) {
            $values['mem_total'] = (int) $m[1];
        }
        if (preg_match('/MemFree:\s*(\d+)/i', $content, $m)) {
            $values['mem_free'] = (int) $m[1];
        }
        if (preg_match('/MemAvailable:\s*(\d+)/i', $content, $m)) {
            $values['mem_available'] = (int) $m[1];
        }
        if (preg_match('/SwapTotal:\s*(\d+)/i', $content, $m)) {
            $values['swap_total'] = (int) $m[1];
        }
        if (preg_match('/SwapFree:\s*(\d+)/i', $content, $m)) {
            $values['swap_free'] = (int) $m[1];
        }

        $memTotal = $values['mem_total'] ?? 0;
        $memAvailable = $values['mem_available'] ?? ($values['mem_free'] ?? 0);
        $memUsed = $memTotal - $memAvailable;

        $swapTotal = $values['swap_total'] ?? 0;
        $swapFree = $values['swap_free'] ?? 0;
        $swapUsed = $swapTotal - $swapFree;

        return [
            [
                'type' => 'RAM',
                'total' => $this->formatKb($memTotal),
                'used' => $this->formatKb($memUsed),
                'free' => $this->formatKb($memAvailable),
            ],
            [
                'type' => 'Swap',
                'total' => $this->formatKb($swapTotal),
                'used' => $this->formatKb($swapUsed),
                'free' => $this->formatKb($swapFree),
            ],
        ];
    }

    private function formatKb(int $kb): string
    {
        if ($kb >= 1048576) {
            return round($kb / 1048576, 1).' GB';
        }

        return round($kb / 1024).' MB';
    }
}
