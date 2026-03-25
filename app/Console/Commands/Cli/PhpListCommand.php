<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class PhpListCommand extends JabaliCommand
{
    protected $signature = 'jabali:php:list {--json} {--yes}';

    protected $description = 'List installed PHP versions';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $result = $this->callAgent('php.list_versions');

        if ($result === null) {
            return Command::FAILURE;
        }

        $versions = $result->get('versions', []);

        if ($this->option('json')) {
            $this->formatter()->json($versions);

            return Command::SUCCESS;
        }

        $rows = [];
        foreach ($versions as $version) {
            $active = ($version['active'] ?? false);
            $isDefault = ($version['default'] ?? false);
            $rows[] = [
                $version['version'] ?? 'unknown',
                $active ? '<fg=green>Active</>' : '<fg=red>Inactive</>',
                $isDefault ? 'Yes' : 'No',
            ];
        }

        $this->formatter()->table(['Version', 'Status', 'Default'], $rows);

        return Command::SUCCESS;
    }
}
