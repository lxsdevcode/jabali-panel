<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class BackupListCommand extends JabaliCommand
{
    protected $signature = 'jabali:backup:list {--user=} {--json} {--yes}';

    protected $description = 'List available backups';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $params = [];
        $username = $this->option('user');
        if ($username !== null) {
            $params['username'] = $username;
        }

        $result = $this->callAgent('backup.list', $params);

        if ($result === null) {
            return Command::FAILURE;
        }

        $backups = $result->get('backups', []);

        if ($this->option('json')) {
            $this->formatter()->json($backups);

            return Command::SUCCESS;
        }

        $rows = [];
        foreach ($backups as $backup) {
            $rows[] = [
                $backup['id'] ?? $backup['name'] ?? 'unknown',
                $backup['user'] ?? 'unknown',
                $backup['date'] ?? 'unknown',
                $backup['size'] ?? 'unknown',
                $backup['type'] ?? 'unknown',
            ];
        }

        $this->formatter()->table(['ID/Name', 'User', 'Date', 'Size', 'Type'], $rows);

        return Command::SUCCESS;
    }
}
