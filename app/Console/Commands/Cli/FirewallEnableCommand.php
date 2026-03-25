<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class FirewallEnableCommand extends JabaliCommand
{
    protected $signature = 'jabali:firewall:enable {--json} {--yes}';

    protected $description = 'Enable the firewall';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $result = $this->callAgent('ufw.enable');

        if ($result === null) {
            return Command::FAILURE;
        }

        $this->formatter()->success('Firewall enabled');

        return Command::SUCCESS;
    }
}
