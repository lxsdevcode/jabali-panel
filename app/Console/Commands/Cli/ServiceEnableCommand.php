<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class ServiceEnableCommand extends JabaliCommand
{
    protected $signature = 'jabali:service:enable {name} {--json} {--yes}';

    protected $description = 'Enable a service on boot';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $service = $this->argument('name');
        $result = $this->callAgent('service.enable', ['service' => $service]);

        if ($result === null) {
            return Command::FAILURE;
        }

        $this->formatter()->success("Service '{$service}' enabled on boot");

        return Command::SUCCESS;
    }
}
