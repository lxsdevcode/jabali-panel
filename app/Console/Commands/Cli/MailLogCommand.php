<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class MailLogCommand extends JabaliCommand
{
    protected $signature = 'jabali:mail:log {--lines=50} {--domain=} {--type=} {--json} {--yes}';

    protected $description = 'View email logs';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $params = [
            'lines' => (int) $this->option('lines'),
        ];

        if ($domain = $this->option('domain')) {
            $params['domain'] = $domain;
        }

        if ($type = $this->option('type')) {
            $params['type'] = $type;
        }

        $result = $this->callAgent('email.get_logs', $params);

        if ($result === null) {
            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json($result->data ?? []);

            return Command::SUCCESS;
        }

        $logs = $result->data ?? [];

        if (empty($logs)) {
            $this->formatter()->info('No logs found');

            return Command::SUCCESS;
        }

        if (is_string($logs)) {
            $this->line($logs);

            return Command::SUCCESS;
        }

        $rows = [];
        if (is_array($logs)) {
            foreach ($logs as $log) {
                if (is_array($log)) {
                    $rows[] = [
                        $log['timestamp'] ?? '-',
                        $log['from'] ?? '-',
                        $log['to'] ?? '-',
                        $log['type'] ?? '-',
                        $log['status'] ?? '-',
                        $log['message'] ?? '-',
                    ];
                }
            }
        }

        if (empty($rows)) {
            $this->formatter()->info('No logs found');

            return Command::SUCCESS;
        }

        $this->formatter()->table(['Timestamp', 'From', 'To', 'Type', 'Status', 'Message'], $rows);

        return Command::SUCCESS;
    }
}
