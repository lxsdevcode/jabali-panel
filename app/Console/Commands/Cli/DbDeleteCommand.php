<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class DbDeleteCommand extends JabaliCommand
{
    protected $signature = 'jabali:db:delete {name} {--json} {--yes}';

    protected $description = 'Delete a database';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $name = $this->argument('name');

        if (! $this->confirmAction("Are you sure you want to delete database '{$name}'?")) {
            return Command::SUCCESS;
        }

        $result = $this->callAgent('mysql.delete_database', ['name' => $name]);

        if ($result === null) {
            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json(['deleted' => $name]);

            return Command::SUCCESS;
        }

        $this->formatter()->success("Database '{$name}' deleted");

        return Command::SUCCESS;
    }
}
