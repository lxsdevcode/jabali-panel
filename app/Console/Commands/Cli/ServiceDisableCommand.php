<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class ServiceDisableCommand extends JabaliCommand
{
    protected $signature = 'jabali:service:disable {name} {--json} {--yes}';

    protected $description = 'Disable a service on boot';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $service = $this->argument('name');

        if (! $this->confirmAction("Are you sure you want to disable {$service} on boot?")) {
            return Command::SUCCESS;
        }

        $result = $this->callAgent('service.disable', ['service' => $service]);

        if ($result === null) {
            return Command::FAILURE;
        }

        $this->formatter()->success("Service '{$service}' disabled on boot");

        return Command::SUCCESS;
    }
}
