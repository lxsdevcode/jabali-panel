<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class WpListCommand extends JabaliCommand
{
    protected $signature = 'jabali:wp:list {--user=} {--json} {--yes}';

    protected $description = 'List WordPress installations';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $params = [];

        if ($username = $this->option('user')) {
            $params['username'] = $username;
        }

        $result = $this->callAgent('wp.list', $params);

        if ($result === null) {
            return Command::FAILURE;
        }

        $sites = $result->get('sites', []);

        if ($this->option('json')) {
            $this->formatter()->json($sites);

            return Command::SUCCESS;
        }

        $rows = [];
        foreach ($sites as $site) {
            $status = ($site['status'] ?? 'unknown') === 'active'
                ? '<fg=green>Active</>'
                : '<fg=red>Inactive</>';
            $updates = ($site['updates'] ?? false)
                ? '<fg=yellow>Available</>'
                : '<fg=green>Up to date</>';

            $rows[] = [
                $site['site'] ?? $site['domain'] ?? 'unknown',
                $site['path'] ?? '-',
                $site['version'] ?? '-',
                $status,
                $updates,
            ];
        }

        $this->formatter()->table(['Site', 'Path', 'Version', 'Status', 'Updates'], $rows);

        return Command::SUCCESS;
    }
}
