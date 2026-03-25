<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class PhpInstallCommand extends JabaliCommand
{
    protected $signature = 'jabali:php:install {version} {--json} {--yes}';

    protected $description = 'Install a PHP version';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $version = $this->argument('version');
        $result = $this->callAgent('php.install', ['version' => $version]);

        if ($result === null) {
            return Command::FAILURE;
        }

        $this->formatter()->success("PHP {$version} installed");

        return Command::SUCCESS;
    }
}
