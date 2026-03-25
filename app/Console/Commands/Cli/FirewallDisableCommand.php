<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class FirewallDisableCommand extends JabaliCommand
{
    protected $signature = 'jabali:firewall:disable {--json} {--yes}';

    protected $description = 'Disable the firewall';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        if (! $this->confirmAction('Are you sure you want to disable the firewall?')) {
            return Command::SUCCESS;
        }

        $result = $this->callAgent('ufw.disable');

        if ($result === null) {
            return Command::FAILURE;
        }

        $this->formatter()->success('Firewall disabled');

        return Command::SUCCESS;
    }
}
