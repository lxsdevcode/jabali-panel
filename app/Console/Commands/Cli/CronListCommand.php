<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class CronListCommand extends JabaliCommand
{
    protected $signature = 'jabali:cron:list {--user=} {--json} {--yes}';

    protected $description = 'List cron jobs for a user';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $params = [];

        if ($username = $this->option('user')) {
            $params['username'] = $username;
        }

        $result = $this->callAgent('cron.list', $params);

        if ($result === null) {
            return Command::FAILURE;
        }

        $crons = $result->get('crons', []);

        if ($this->option('json')) {
            $this->formatter()->json($crons);

            return Command::SUCCESS;
        }

        if (empty($crons)) {
            $this->formatter()->info('No cron jobs found');

            return Command::SUCCESS;
        }

        $rows = [];
        foreach ($crons as $index => $cron) {
            $schedule = sprintf(
                '%s %s %s %s %s',
                $cron['minute'] ?? '*',
                $cron['hour'] ?? '*',
                $cron['day'] ?? '*',
                $cron['month'] ?? '*',
                $cron['weekday'] ?? '*'
            );

            $status = isset($cron['enabled']) && $cron['enabled'] ? '<fg=green>Enabled</>' : '<fg=yellow>Disabled</>';

            $rows[] = [
                (string) $index,
                $schedule,
                $cron['command'] ?? 'unknown',
                $status,
            ];
        }

        $this->formatter()->table(['Index', 'Schedule', 'Command', 'Status'], $rows);

        return Command::SUCCESS;
    }
}
