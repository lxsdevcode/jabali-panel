<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use App\Services\Agent\AgentClientInterface;
use App\Services\Agent\AgentResult;
use Illuminate\Console\Command;

class CpanelRestoreCommand extends JabaliCommand
{
    protected $signature = 'jabali:cpanel:restore {path} {--user=} {--json} {--yes}';

    protected $description = 'Restore a cPanel backup';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $path = $this->argument('path');
        $username = $this->option('user');

        if (! $username) {
            $this->formatter()->error('The --user option is required');

            return Command::FAILURE;
        }

        if (! $this->confirmAction("Are you sure you want to restore cPanel backup '{$path}' for user '{$username}'?")) {
            return Command::SUCCESS;
        }

        $this->formatter()->info('Restoring cPanel backup... This may take several minutes.');

        $agent = app(AgentClientInterface::class);
        $response = $agent->send('cpanel.restore_backup', [
            'path' => $path,
            'username' => $username,
        ]);
        $result = AgentResult::fromResponse($response);

        if ($result->failed()) {
            $this->formatter()->error($result->error ?? 'cPanel restore failed');

            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json($result->toArray());

            return Command::SUCCESS;
        }

        $this->formatter()->success("cPanel backup restored for '{$username}'");

        return Command::SUCCESS;
    }
}
