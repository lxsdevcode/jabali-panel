<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class DbUsersCommand extends JabaliCommand
{
    protected $signature = 'jabali:db:users {--json} {--yes}';

    protected $description = 'List database users';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $result = $this->callAgent('mysql.list_users');

        if ($result === null) {
            return Command::FAILURE;
        }

        $users = $result->get('users', []);

        if ($this->option('json')) {
            $this->formatter()->json($users);

            return Command::SUCCESS;
        }

        $rows = [];
        foreach ($users as $user) {
            $rows[] = [
                $user['user'] ?? 'unknown',
                $user['host'] ?? '-',
                $user['databases'] ?? '-',
            ];
        }

        $this->formatter()->table(['User', 'Host', 'Databases'], $rows);

        return Command::SUCCESS;
    }
}
