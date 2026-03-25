<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class SslListCommand extends JabaliCommand
{
    protected $signature = 'jabali:ssl:list {--json} {--yes}';

    protected $description = 'List SSL certificates and their status';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $result = $this->callAgent('ssl.list');

        if ($result === null) {
            return Command::FAILURE;
        }

        $certificates = $result->get('certificates', []);

        if ($this->option('json')) {
            $this->formatter()->json($certificates);

            return Command::SUCCESS;
        }

        $rows = [];
        foreach ($certificates as $cert) {
            $valid = $cert['valid'] ?? false;
            $days = $cert['days_remaining'] ?? 0;

            $rows[] = [
                $cert['domain'] ?? 'unknown',
                $valid ? '<fg=green>Valid</>' : '<fg=red>Invalid</>',
                $cert['issuer'] ?? 'unknown',
                $cert['valid_to'] ?? 'unknown',
                $this->colorDays($days),
            ];
        }

        $this->formatter()->table(['Domain', 'Status', 'Issuer', 'Expires', 'Days'], $rows);

        return Command::SUCCESS;
    }

    private function colorDays(int $days): string
    {
        if ($days > 30) {
            return "<fg=green>{$days}</>";
        }

        if ($days > 7) {
            return "<fg=yellow>{$days}</>";
        }

        return "<fg=red>{$days}</>";
    }
}
