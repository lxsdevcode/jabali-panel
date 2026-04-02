<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class CronCreateCommand extends JabaliCommand
{
    protected $signature = 'jabali:cron:create {--user=} {--schedule=} {--command=} {--json} {--yes}';

    protected $description = 'Create a cron job';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $username = $this->option('user') ?? $this->ask('Username');
        $schedule = $this->option('schedule') ?? $this->ask('Schedule (cron format: minute hour day month weekday)');
        $command = $this->option('command') ?? $this->ask('Command to execute');

        // Parse the schedule string
        $parts = explode(' ', trim($schedule));

        if (count($parts) !== 5) {
            $this->formatter()->error('Schedule must contain exactly 5 parts (minute hour day month weekday)');

            return Command::FAILURE;
        }

        $params = [
            'username' => $username,
            'minute' => $parts[0],
            'hour' => $parts[1],
            'day' => $parts[2],
            'month' => $parts[3],
            'weekday' => $parts[4],
            'command' => $command,
        ];

        $result = $this->callAgent('cron.create', $params);

        if ($result === null) {
            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json($result->toArray());

            return Command::SUCCESS;
        }

        $this->formatter()->success("Cron job created: {$schedule} {$command}");

        return Command::SUCCESS;
    }
}
