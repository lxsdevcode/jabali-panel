<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class PhpUninstallCommand extends JabaliCommand
{
    protected $signature = 'jabali:php:uninstall {version} {--json} {--yes}';

    protected $description = 'Uninstall a PHP version';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $version = $this->argument('version');

        if (! $this->confirmAction("Are you sure you want to uninstall PHP {$version}?")) {
            return Command::SUCCESS;
        }

        $result = $this->callAgent('php.uninstall', ['version' => $version]);

        if ($result === null) {
            return Command::FAILURE;
        }

        $this->formatter()->success("PHP {$version} uninstalled");

        return Command::SUCCESS;
    }
}
