<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class SslStatusCommand extends JabaliCommand
{
    protected $signature = 'jabali:ssl:status {domain} {--json} {--yes}';

    protected $description = 'Show SSL certificate status for a domain';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $domain = $this->argument('domain');
        $result = $this->callAgent('ssl.info', ['domain' => $domain]);

        if ($result === null) {
            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json($result->toArray());

            return Command::SUCCESS;
        }

        $valid = $result->get('valid', false);
        $days = (int) $result->get('days_remaining', 0);

        $status = $valid ? '<fg=green>Valid</>' : '<fg=red>Invalid</>';
        $daysColored = $this->colorDays($days);

        $this->formatter()->info("Domain:    {$domain}");
        $this->formatter()->info("Status:    {$status}");
        $this->formatter()->info("Issuer:    {$result->get('issuer', 'unknown')}");
        $this->formatter()->info("From:      {$result->get('valid_from', 'unknown')}");
        $this->formatter()->info("To:        {$result->get('valid_to', 'unknown')}");
        $this->formatter()->info("Days left: {$daysColored}");

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
