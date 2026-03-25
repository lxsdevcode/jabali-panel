<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use App\Services\Agent\AgentClientInterface;
use App\Services\Agent\AgentResult;
use Illuminate\Console\Command;

class BackupCreateCommand extends JabaliCommand
{
    protected $signature = 'jabali:backup:create {username} {--json} {--yes}';

    protected $description = 'Create a backup for a user';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $username = $this->argument('username');

        $this->formatter()->info("Creating backup for <fg=green>{$username}</>... This may take several minutes.");

        $agent = app(AgentClientInterface::class);
        $response = $agent->send('backup.create', ['username' => $username]);
        $result = AgentResult::fromResponse($response);

        if ($result->failed()) {
            $this->formatter()->error($result->error ?? 'Backup creation failed');

            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json($result->toArray());

            return Command::SUCCESS;
        }

        $this->formatter()->success("Backup created for '{$username}'");

        return Command::SUCCESS;
    }
}
