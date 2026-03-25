<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class WpScanCommand extends JabaliCommand
{
    protected $signature = 'jabali:wp:scan {domain} {--json} {--yes}';

    protected $description = 'Scan a WordPress installation for issues';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $domain = $this->argument('domain');

        $result = $this->callAgent('wp.scan', ['domain' => $domain]);

        if ($result === null) {
            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json($result->toArray());

            return Command::SUCCESS;
        }

        $this->formatter()->success("Scan complete for {$domain}");

        return Command::SUCCESS;
    }
}
