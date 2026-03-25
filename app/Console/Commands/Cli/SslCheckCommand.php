<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Support\Facades\Artisan;

class SslCheckCommand extends JabaliCommand
{
    protected $signature = 'jabali:ssl:check {domain?} {--issue-only} {--renew-only} {--json} {--yes}';

    protected $description = 'Check SSL certificates and issue/renew as needed';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $params = [];

        $domain = $this->argument('domain');
        if ($domain !== null) {
            $params['--domain'] = $domain;
        }

        if ($this->option('issue-only')) {
            $params['--issue-only'] = true;
        }

        if ($this->option('renew-only')) {
            $params['--renew-only'] = true;
        }

        return Artisan::call('jabali:ssl-check', $params, $this->getOutput());
    }
}
