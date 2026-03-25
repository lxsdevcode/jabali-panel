<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class WpDeleteCommand extends JabaliCommand
{
    protected $signature = 'jabali:wp:delete {domain} {--json} {--yes}';

    protected $description = 'Delete a WordPress installation';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $domain = $this->argument('domain');

        if (! $this->confirmAction("Are you sure you want to delete WordPress on '{$domain}'?")) {
            return Command::SUCCESS;
        }

        $result = $this->callAgent('wp.delete', ['domain' => $domain]);

        if ($result === null) {
            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json(['deleted' => $domain]);

            return Command::SUCCESS;
        }

        $this->formatter()->success("WordPress deleted from {$domain}");

        return Command::SUCCESS;
    }
}
