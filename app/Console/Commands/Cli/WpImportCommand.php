<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class WpImportCommand extends JabaliCommand
{
    protected $signature = 'jabali:wp:import {path} {--user=} {--json} {--yes}';

    protected $description = 'Import a WordPress installation from a path';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $path = $this->argument('path');

        $params = ['path' => $path];

        if ($username = $this->option('user')) {
            $params['username'] = $username;
        }

        $result = $this->callAgent('wp.import', $params);

        if ($result === null) {
            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json($result->toArray());

            return Command::SUCCESS;
        }

        $this->formatter()->success("WordPress imported from {$path}");

        return Command::SUCCESS;
    }
}
