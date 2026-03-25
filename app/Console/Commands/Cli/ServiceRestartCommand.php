<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class ServiceRestartCommand extends JabaliCommand
{
    protected $signature = 'jabali:service:restart {name} {--json} {--yes}';

    protected $description = 'Restart a service';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $service = $this->argument('name');
        $result = $this->callAgent('service.restart', ['service' => $service]);

        if ($result === null) {
            return Command::FAILURE;
        }

        $this->formatter()->success("Service '{$service}' restarted");

        return Command::SUCCESS;
    }
}
