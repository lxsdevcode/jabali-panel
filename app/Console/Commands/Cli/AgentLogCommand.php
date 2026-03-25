<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class AgentLogCommand extends JabaliCommand
{
    protected $signature = 'jabali:agent:log {--lines=50} {--json} {--yes}';

    protected $description = 'Show recent agent log entries';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $lines = (int) $this->option('lines');

        if ($lines < 1) {
            $lines = 50;
        }

        $logPath = storage_path('logs/agent.log');

        if (! file_exists($logPath)) {
            $this->formatter()->error("Agent log not found: {$logPath}");

            return Command::FAILURE;
        }

        $process = new \Symfony\Component\Process\Process(['tail', '-n', (string) $lines, $logPath]);
        $process->run();

        $content = trim($process->getOutput());

        if ($this->option('json')) {
            $this->formatter()->json([
                'file' => $logPath,
                'lines' => $lines,
                'content' => $content,
            ]);

            return Command::SUCCESS;
        }

        $this->formatter()->info($content);

        return Command::SUCCESS;
    }
}
