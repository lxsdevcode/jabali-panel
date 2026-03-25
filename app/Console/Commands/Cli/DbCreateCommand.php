<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class DbCreateCommand extends JabaliCommand
{
    protected $signature = 'jabali:db:create {name} {--user=} {--json} {--yes}';

    protected $description = 'Create a database';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $name = $this->argument('name');

        $params = ['name' => $name];

        if ($username = $this->option('user')) {
            $params['username'] = $username;
        }

        $result = $this->callAgent('mysql.create_database', $params);

        if ($result === null) {
            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json($result->toArray());

            return Command::SUCCESS;
        }

        $this->formatter()->success("Database '{$name}' created");

        return Command::SUCCESS;
    }
}
