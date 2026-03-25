<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class DbListCommand extends JabaliCommand
{
    protected $signature = 'jabali:db:list {--user=} {--json} {--yes}';

    protected $description = 'List databases';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $params = [];

        if ($username = $this->option('user')) {
            $params['username'] = $username;
        }

        $result = $this->callAgent('mysql.list_databases', $params);

        if ($result === null) {
            return Command::FAILURE;
        }

        $databases = $result->get('databases', []);

        if ($this->option('json')) {
            $this->formatter()->json($databases);

            return Command::SUCCESS;
        }

        $rows = [];
        foreach ($databases as $db) {
            $rows[] = [
                $db['name'] ?? 'unknown',
                $db['size'] ?? '-',
                $db['tables'] ?? '-',
                $db['user'] ?? '-',
            ];
        }

        $this->formatter()->table(['Name', 'Size', 'Tables', 'User'], $rows);

        return Command::SUCCESS;
    }
}
