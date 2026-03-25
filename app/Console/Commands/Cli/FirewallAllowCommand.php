<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class FirewallAllowCommand extends JabaliCommand
{
    protected $signature = 'jabali:firewall:allow {port} {--proto=} {--from=} {--json} {--yes}';

    protected $description = 'Allow a port through the firewall';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $params = ['port' => $this->argument('port')];

        if ($proto = $this->option('proto')) {
            $params['proto'] = $proto;
        }

        if ($from = $this->option('from')) {
            $params['from'] = $from;
        }

        $result = $this->callAgent('ufw.allow_port', $params);

        if ($result === null) {
            return Command::FAILURE;
        }

        $this->formatter()->success("Port {$params['port']} allowed");

        return Command::SUCCESS;
    }
}
