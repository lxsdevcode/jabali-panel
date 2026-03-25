<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class PhpDefaultCommand extends JabaliCommand
{
    protected $signature = 'jabali:php:default {version} {--json} {--yes}';

    protected $description = 'Set the default PHP version';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $version = $this->argument('version');
        $result = $this->callAgent('php.set_default', ['version' => $version]);

        if ($result === null) {
            return Command::FAILURE;
        }

        $this->formatter()->success("PHP {$version} set as default");

        return Command::SUCCESS;
    }
}
