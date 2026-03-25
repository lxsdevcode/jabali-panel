<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class SslRenewCommand extends JabaliCommand
{
    protected $signature = 'jabali:ssl:renew {domain} {--json} {--yes}';

    protected $description = 'Renew an SSL certificate for a domain';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $domain = $this->argument('domain');
        $result = $this->callAgent('ssl.renew', ['domain' => $domain]);

        if ($result === null) {
            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json($result->toArray());

            return Command::SUCCESS;
        }

        $expiresAt = $result->get('valid_to', 'unknown');
        $this->formatter()->success("SSL certificate renewed for {$domain} (expires: {$expiresAt})");

        return Command::SUCCESS;
    }
}
