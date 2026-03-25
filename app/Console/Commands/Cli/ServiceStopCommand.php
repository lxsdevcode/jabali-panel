<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class ServiceStopCommand extends JabaliCommand
{
    protected $signature = 'jabali:service:stop {name} {--json} {--yes}';

    protected $description = 'Stop a service';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $service = $this->argument('name');

        if (! $this->confirmAction("Are you sure you want to stop {$service}?")) {
            return Command::SUCCESS;
        }

        $result = $this->callAgent('service.stop', ['service' => $service]);

        if ($result === null) {
            return Command::FAILURE;
        }

        $this->formatter()->success("Service '{$service}' stopped");

        return Command::SUCCESS;
    }
}
