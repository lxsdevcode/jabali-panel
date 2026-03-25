<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class WpUpdateCommand extends JabaliCommand
{
    protected $signature = 'jabali:wp:update {domain} {--json} {--yes}';

    protected $description = 'Update a WordPress installation';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $domain = $this->argument('domain');

        $result = $this->callAgent('wp.update', ['domain' => $domain]);

        if ($result === null) {
            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json($result->toArray());

            return Command::SUCCESS;
        }

        $this->formatter()->success("WordPress updated on {$domain}");

        return Command::SUCCESS;
    }
}
