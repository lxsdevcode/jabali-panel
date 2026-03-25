<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class SystemHostnameCommand extends JabaliCommand
{
    protected $signature = 'jabali:system:hostname {name?} {--json} {--yes}';

    protected $description = 'Show or set the system hostname';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $name = $this->argument('name');

        if ($name !== null) {
            return $this->setHostname($name);
        }

        return $this->showHostname();
    }

    private function showHostname(): int
    {
        $hostname = gethostname() ?: 'unknown';

        if ($this->option('json')) {
            $this->formatter()->json(['hostname' => $hostname]);

            return Command::SUCCESS;
        }

        $this->formatter()->info("Hostname: <fg=green>{$hostname}</>");

        return Command::SUCCESS;
    }

    private function setHostname(string $name): int
    {
        $result = $this->callAgent('server.set_hostname', ['hostname' => $name]);

        if ($result === null) {
            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json(['hostname' => $name]);

            return Command::SUCCESS;
        }

        $this->formatter()->success("Hostname set to '{$name}'");

        return Command::SUCCESS;
    }
}
