<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class CpanelFixPermissionsCommand extends JabaliCommand
{
    protected $signature = 'jabali:cpanel:fix-permissions {username} {--json} {--yes}';

    protected $description = 'Fix file permissions for a user after cPanel migration';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $username = $this->argument('username');
        $result = $this->callAgent('cpanel.fix_permissions', ['username' => $username]);

        if ($result === null) {
            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json($result->toArray());

            return Command::SUCCESS;
        }

        $this->formatter()->success("Permissions fixed for '{$username}'");

        return Command::SUCCESS;
    }
}
