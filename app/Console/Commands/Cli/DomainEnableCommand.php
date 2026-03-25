<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use App\Models\Domain;
use Illuminate\Console\Command;

class DomainEnableCommand extends JabaliCommand
{
    protected $signature = 'jabali:domain:enable {domain} {--json} {--yes}';

    protected $description = 'Enable a domain';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $domainName = $this->argument('domain');
        $domain = Domain::where('domain', $domainName)->first();

        if (! $domain) {
            $this->formatter()->error("Domain not found: {$domainName}");

            return Command::FAILURE;
        }

        $domain->update(['is_active' => true]);

        if ($this->option('json')) {
            $this->formatter()->json(['domain' => $domainName, 'is_active' => true]);

            return Command::SUCCESS;
        }

        $this->formatter()->success("Domain '{$domainName}' enabled");

        return Command::SUCCESS;
    }
}
