<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class ServiceStatusCommand extends JabaliCommand
{
    protected $signature = 'jabali:service:status {name} {--json} {--yes}';

    protected $description = 'Show status of a specific service';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $service = $this->argument('name');
        $result = $this->callAgent('service.list', ['services' => [$service]]);

        if ($result === null) {
            return Command::FAILURE;
        }

        $services = $result->get('services', []);
        $status = $services[$service] ?? null;

        if (! $status) {
            $this->formatter()->error("Service not found: {$service}");

            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json([$service => $status]);

            return Command::SUCCESS;
        }

        $active = ($status['is_active'] ?? false) ? 'Running' : 'Stopped';
        $enabled = ($status['is_enabled'] ?? false) ? 'Enabled' : 'Disabled';
        $this->formatter()->info("{$service}: {$active} (Boot: {$enabled})");

        return Command::SUCCESS;
    }
}
