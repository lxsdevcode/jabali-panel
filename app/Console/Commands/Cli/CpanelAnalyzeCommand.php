<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class CpanelAnalyzeCommand extends JabaliCommand
{
    protected $signature = 'jabali:cpanel:analyze {path} {--json} {--yes}';

    protected $description = 'Analyze a cPanel backup';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $path = $this->argument('path');

        if (! file_exists($path)) {
            $this->formatter()->error("File not found: {$path}");

            return Command::FAILURE;
        }

        $result = $this->callAgent('cpanel.analyze_backup', ['path' => $path]);

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
