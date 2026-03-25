<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class BackupDeleteCommand extends JabaliCommand
{
    protected $signature = 'jabali:backup:delete {path} {--json} {--yes}';

    protected $description = 'Delete a backup';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $path = $this->argument('path');

        if (! $this->confirmAction("Are you sure you want to delete backup '{$path}'?")) {
            return Command::SUCCESS;
        }

        $result = $this->callAgent('backup.delete', ['path' => $path]);

        if ($result === null) {
            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json(['deleted' => $path]);

            return Command::SUCCESS;
        }

        $this->formatter()->success("Backup '{$path}' deleted");

        return Command::SUCCESS;
    }
}
