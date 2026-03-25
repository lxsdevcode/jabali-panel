<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class ServiceStartCommand extends JabaliCommand
{
    protected $signature = 'jabali:service:start {name} {--json} {--yes}';

    protected $description = 'Start a service';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $service = $this->argument('name');
        $result = $this->callAgent('service.start', ['service' => $service]);

        if ($result === null) {
            return Command::FAILURE;
        }

        $this->formatter()->success("Service '{$service}' started");

        return Command::SUCCESS;
    }
}
