<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Support\Facades\Artisan;

class ReportCommand extends JabaliCommand
{
    protected $signature = 'jabali:report:generate {--file} {--json} {--yes}';

    protected $description = 'Generate a diagnostic report';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $params = [];

        if ($this->option('file')) {
            $params['--file'] = true;
        }

        if ($this->option('json')) {
            $params['--json'] = true;
        }

        return Artisan::call('jabali:report', $params, $output);
    }
}
