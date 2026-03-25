<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use App\Models\Domain;
use Illuminate\Console\Command;

class DomainDeleteCommand extends JabaliCommand
{
    protected $signature = 'jabali:domain:delete {domain} {--json} {--yes}';

    protected $description = 'Delete a domain';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $domainName = $this->argument('domain');
        $domain = Domain::with('user')->where('domain', $domainName)->first();

        if (! $domain) {
            $this->formatter()->error("Domain not found: {$domainName}");

            return Command::FAILURE;
        }

        if (! $this->confirmAction("Are you sure you want to delete domain '{$domainName}'?")) {
            return Command::SUCCESS;
        }

        $username = $domain->user?->username;

        $result = $this->callAgent('domain.delete', [
            'username' => $username,
            'domain' => $domainName,
        ]);

        if ($result === null) {
            return Command::FAILURE;
        }

        $domain->delete();

        if ($this->option('json')) {
            $this->formatter()->json(['deleted' => $domainName]);

            return Command::SUCCESS;
        }

        $this->formatter()->success("Domain '{$domainName}' deleted");

        return Command::SUCCESS;
    }
}
