<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class DbUserCreateCommand extends JabaliCommand
{
    protected $signature = 'jabali:db:user-create {name} {--password=} {--database=} {--json} {--yes}';

    protected $description = 'Create a database user';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $name = $this->argument('name');

        $params = ['name' => $name];

        if ($password = $this->option('password')) {
            $params['password'] = $password;
        }

        if ($database = $this->option('database')) {
            $params['database'] = $database;
        }

        $result = $this->callAgent('mysql.create_user', $params);

        if ($result === null) {
            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json($result->toArray());

            return Command::SUCCESS;
        }

        $this->formatter()->success("Database user '{$name}' created");

        return Command::SUCCESS;
    }
}
