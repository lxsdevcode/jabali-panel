<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class SslIssueCommand extends JabaliCommand
{
    protected $signature = 'jabali:ssl:issue {domain} {--force} {--json} {--yes}';

    protected $description = 'Issue an SSL certificate for a domain';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $domain = $this->argument('domain');
        $params = ['domain' => $domain];

        if ($this->option('force')) {
            $params['force'] = true;
        }

        $result = $this->callAgent('ssl.issue', $params);

        if ($result === null) {
            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json($result->toArray());

            return Command::SUCCESS;
        }

        $expiresAt = $result->get('valid_to', 'unknown');
        $this->formatter()->success("SSL certificate issued for {$domain} (expires: {$expiresAt})");

        return Command::SUCCESS;
    }
}
