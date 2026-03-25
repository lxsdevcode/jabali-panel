<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class SystemStatusCommand extends JabaliCommand
{
    protected $signature = 'jabali:system:status {--json} {--yes}';

    protected $description = 'Show system status and key metrics';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $result = $this->callAgent('metrics.overview');

        if ($result === null) {
            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json($result->toArray());

            return Command::SUCCESS;
        }

        $rows = [];
        foreach ($result->toArray() as $key => $value) {
            if ($key === 'success') {
                continue;
            }

            $rows[] = [$key, is_array($value) ? json_encode($value) : (string) $value];
        }

        $this->formatter()->table(['Metric', 'Value'], $rows);

        return Command::SUCCESS;
    }
}
