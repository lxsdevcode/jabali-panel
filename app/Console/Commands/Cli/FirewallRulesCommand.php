<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class FirewallRulesCommand extends JabaliCommand
{
    protected $signature = 'jabali:firewall:rules {--json} {--yes}';

    protected $description = 'List firewall rules';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $result = $this->callAgent('ufw.list_rules');

        if ($result === null) {
            return Command::FAILURE;
        }

        $rules = $result->get('rules', []);

        if ($this->option('json')) {
            $this->formatter()->json($rules);

            return Command::SUCCESS;
        }

        $rows = [];
        foreach ($rules as $rule) {
            $rows[] = [
                $rule['number'] ?? '-',
                $rule['to'] ?? '-',
                $rule['action'] ?? '-',
                $rule['from'] ?? '-',
            ];
        }

        $this->formatter()->table(['Number', 'To', 'Action', 'From'], $rows);

        return Command::SUCCESS;
    }
}
