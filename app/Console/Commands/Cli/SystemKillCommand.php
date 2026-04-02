<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class SystemKillCommand extends JabaliCommand
{
    protected $signature = 'jabali:system:kill {pid} {--signal=TERM} {--json} {--yes}';

    protected $description = 'Kill a system process';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $pid = (int) $this->argument('pid');
        $signal = $this->option('signal');

        if ($pid <= 0) {
            $this->formatter()->error('Invalid PID: must be greater than 0');

            return Command::FAILURE;
        }

        if (! $this->confirmAction("Kill process {$pid} with signal {$signal}?")) {
            return Command::SUCCESS;
        }

        $result = $this->callAgent('system.kill_process', [
            'pid' => $pid,
            'signal' => $signal,
        ]);

        if ($result === null) {
            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json(['pid' => $pid, 'signal' => $signal, 'killed' => true]);

            return Command::SUCCESS;
        }

        $this->formatter()->success("Process {$pid} killed with signal {$signal}");

        return Command::SUCCESS;
    }
}
