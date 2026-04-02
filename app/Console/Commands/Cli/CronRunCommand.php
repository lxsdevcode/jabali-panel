<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class CronRunCommand extends JabaliCommand
{
    protected $signature = 'jabali:cron:run {index} {--user=} {--json} {--yes}';

    protected $description = 'Run a cron job immediately';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $index = $this->argument('index');
        $username = $this->option('user') ?? $this->ask('Username');

        $params = [
            'username' => $username,
            'index' => (int) $index,
        ];

        $result = $this->callAgent('cron.run', $params);

        if ($result === null) {
            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json($result->toArray());

            return Command::SUCCESS;
        }

        $this->formatter()->success('Cron job executed');

        if ($output = $result->get('output')) {
            $this->formatter()->info('Output:');
            $this->formatter()->text($output);
        }

        return Command::SUCCESS;
    }
}
