<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class AgentStatusCommand extends JabaliCommand
{
    protected $signature = 'jabali:agent:status {--json} {--yes}';

    protected $description = 'Show agent process status';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $pidFile = '/var/run/jabali/agent.pid';
        $running = false;
        $pid = null;
        $uptime = null;

        if (file_exists($pidFile)) {
            $pid = (int) trim((string) file_get_contents($pidFile));

            if ($pid > 0 && file_exists("/proc/{$pid}")) {
                $running = true;

                $statFile = "/proc/{$pid}/stat";
                if (file_exists($statFile) && file_exists('/proc/uptime')) {
                    $stat = file_get_contents($statFile);
                    $parts = explode(' ', (string) $stat);
                    $startTicks = (int) ($parts[21] ?? 0);
                    $clkTck = 100; // standard on Linux

                    if ($startTicks > 0) {
                        $systemUptime = (float) explode(' ', (string) file_get_contents('/proc/uptime'))[0];
                        $processSeconds = $systemUptime - ($startTicks / $clkTck);
                        $uptime = gmdate('H:i:s', (int) max(0, $processSeconds));
                    }
                }
            }
        }

        $data = [
            'status' => $running ? 'running' : 'stopped',
            'pid' => $pid,
            'uptime' => $uptime,
        ];

        if ($this->option('json')) {
            $this->formatter()->json($data);

            return Command::SUCCESS;
        }

        $status = $running ? '<fg=green>Running</>' : '<fg=red>Stopped</>';
        $this->formatter()->info("Status: {$status}");
        $this->formatter()->info('PID: '.($pid ?? '-'));
        $this->formatter()->info('Uptime: '.($uptime ?? '-'));

        return Command::SUCCESS;
    }
}
