<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class CronToggleCommand extends JabaliCommand
{
    protected $signature = 'jabali:cron:toggle {index} {--user=} {--enable} {--disable} {--json} {--yes}';

    protected $description = 'Enable or disable a cron job';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $index = $this->argument('index');
        $username = $this->option('user') ?? $this->ask('Username');

        $enabled = null;
        if ($this->option('enable')) {
            $enabled = true;
        } elseif ($this->option('disable')) {
            $enabled = false;
        } else {
            $enabled = $this->confirm('Enable cron job?', true);
        }

        $params = [
            'username' => $username,
            'index' => (int) $index,
            'enabled' => $enabled,
        ];

        $result = $this->callAgent('cron.toggle', $params);

        if ($result === null) {
            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json($result->toArray());

            return Command::SUCCESS;
        }

        $status = $enabled ? 'enabled' : 'disabled';
        $this->formatter()->success("Cron job {$status}");

        return Command::SUCCESS;
    }
}
