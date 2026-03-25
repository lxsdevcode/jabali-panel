<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class WpInstallCommand extends JabaliCommand
{
    protected $signature = 'jabali:wp:install {domain} {--user=} {--title=} {--admin-user=} {--admin-email=} {--admin-password=} {--json} {--yes}';

    protected $description = 'Install WordPress on a domain';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $domain = $this->argument('domain');

        $params = ['domain' => $domain];

        if ($username = $this->option('user')) {
            $params['username'] = $username;
        }

        if ($title = $this->option('title')) {
            $params['title'] = $title;
        }

        if ($adminUser = $this->option('admin-user')) {
            $params['admin_user'] = $adminUser;
        }

        if ($adminEmail = $this->option('admin-email')) {
            $params['admin_email'] = $adminEmail;
        }

        if ($adminPassword = $this->option('admin-password')) {
            $params['admin_password'] = $adminPassword;
        }

        $result = $this->callAgent('wp.install', $params);

        if ($result === null) {
            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json($result->toArray());

            return Command::SUCCESS;
        }

        $this->formatter()->success("WordPress installed on {$domain}");

        return Command::SUCCESS;
    }
}
