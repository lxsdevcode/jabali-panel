<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class CronDeleteCommand extends JabaliCommand
{
    protected $signature = 'jabali:cron:delete {index} {--user=} {--json} {--yes}';

    protected $description = 'Delete a cron job';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $index = $this->argument('index');
        $username = $this->option('user') ?? $this->ask('Username');

        if (! $this->confirmAction("Delete cron job at index {$index}?")) {
            $this->formatter()->info('Cancelled');

            return Command::SUCCESS;
        }

        $params = [
            'username' => $username,
            'index' => (int) $index,
        ];

        $result = $this->callAgent('cron.delete', $params);

        if ($result === null) {
            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json($result->toArray());

            return Command::SUCCESS;
        }

        $this->formatter()->success('Cron job deleted');

        return Command::SUCCESS;
    }
}
