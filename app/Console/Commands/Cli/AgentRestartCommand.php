<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;
use Symfony\Component\Process\Process;

class AgentRestartCommand extends JabaliCommand
{
    protected $signature = 'jabali:agent:restart {--json} {--yes}';

    protected $description = 'Restart the agent service';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $process = new Process(['systemctl', 'restart', 'jabali-agent']);
        $process->setTimeout(30);
        $process->run();

        if (! $process->isSuccessful()) {
            $this->formatter()->error('Failed to restart agent: '.trim($process->getErrorOutput()));

            return Command::FAILURE;
        }

        $this->formatter()->success('Agent restarted');

        return Command::SUCCESS;
    }
}
