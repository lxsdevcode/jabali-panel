<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class BackupRestoreCommand extends JabaliCommand
{
    protected $signature = 'jabali:backup:restore {path} {--user=} {--json} {--yes}';

    protected $description = 'Restore a backup from a given path';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $path = $this->argument('path');
        $username = $this->option('user');

        if (! $this->confirmAction("Are you sure you want to restore backup '{$path}'?")) {
            return Command::SUCCESS;
        }

        $params = ['path' => $path];
        if ($username !== null) {
            $params['username'] = $username;
        }

        $result = $this->callAgent('backup.restore', $params);

        if ($result === null) {
            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json($result->toArray());

            return Command::SUCCESS;
        }

        $this->formatter()->success("Backup restored from '{$path}'");

        return Command::SUCCESS;
    }
}
