<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class BackupInfoCommand extends JabaliCommand
{
    protected $signature = 'jabali:backup:info {path} {--json} {--yes}';

    protected $description = 'Show information about a backup';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $path = $this->argument('path');
        $result = $this->callAgent('backup.get_info', ['path' => $path]);

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

        $this->formatter()->table(['Field', 'Value'], $rows);

        return Command::SUCCESS;
    }
}
