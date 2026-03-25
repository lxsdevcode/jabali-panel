<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class FirewallDeleteCommand extends JabaliCommand
{
    protected $signature = 'jabali:firewall:delete {rule_number} {--json} {--yes}';

    protected $description = 'Delete a firewall rule';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $ruleNumber = $this->argument('rule_number');

        if (! $this->confirmAction("Are you sure you want to delete firewall rule #{$ruleNumber}?")) {
            return Command::SUCCESS;
        }

        $result = $this->callAgent('ufw.delete_rule', ['rule_number' => $ruleNumber]);

        if ($result === null) {
            return Command::FAILURE;
        }

        $this->formatter()->success("Firewall rule #{$ruleNumber} deleted");

        return Command::SUCCESS;
    }
}
