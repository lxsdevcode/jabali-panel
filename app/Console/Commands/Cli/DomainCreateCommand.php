<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use App\Models\Domain;
use App\Models\User;
use Illuminate\Console\Command;

class DomainCreateCommand extends JabaliCommand
{
    protected $signature = 'jabali:domain:create {domain} {--user=} {--json} {--yes}';

    protected $description = 'Create a new domain';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $domainName = $this->argument('domain');
        $username = $this->option('user');

        if (! $username) {
            $this->formatter()->error('The --user option is required');

            return Command::FAILURE;
        }

        $user = User::where('username', $username)->first();

        if (! $user) {
            $this->formatter()->error("User not found: {$username}");

            return Command::FAILURE;
        }

        $result = $this->callAgent('domain.create', [
            'username' => $username,
            'domain' => $domainName,
        ]);

        if ($result === null) {
            return Command::FAILURE;
        }

        $domain = Domain::create([
            'user_id' => $user->id,
            'domain' => $domainName,
            'is_active' => true,
        ]);

        if ($this->option('json')) {
            $this->formatter()->json([
                'id' => $domain->id,
                'domain' => $domain->domain,
                'user' => $username,
                'is_active' => $domain->is_active,
                'ssl_enabled' => $domain->ssl_enabled,
                'created_at' => $domain->created_at?->toDateTimeString(),
            ]);

            return Command::SUCCESS;
        }

        $this->formatter()->success("Domain '{$domainName}' created for user '{$username}'");

        return Command::SUCCESS;
    }
}
