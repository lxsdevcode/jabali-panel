<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class FirewallStatusCommand extends JabaliCommand
{
    protected $signature = 'jabali:firewall:status {--json} {--yes}';

    protected $description = 'Show firewall status';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $result = $this->callAgent('ufw.status');

        if ($result === null) {
            return Command::FAILURE;
        }

        $data = $result->toArray();

        if ($this->option('json')) {
            $this->formatter()->json($data);

            return Command::SUCCESS;
        }

        $active = ($data['active'] ?? false);
        $status = $active ? '<fg=green>Active</>' : '<fg=red>Inactive</>';
        $defaultPolicy = $data['default_policy'] ?? 'unknown';
        $rulesCount = $data['rules_count'] ?? 0;

        $this->formatter()->info("Status: {$status}");
        $this->formatter()->info("Default policy: {$defaultPolicy}");
        $this->formatter()->info("Rules: {$rulesCount}");

        return Command::SUCCESS;
    }
}
