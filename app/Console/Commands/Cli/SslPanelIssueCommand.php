<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class SslPanelIssueCommand extends JabaliCommand
{
    protected $signature = 'jabali:ssl:panel-issue {--json} {--yes}';

    protected $description = 'Issue or reinstall Let\'s Encrypt certificate for the panel';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $hostname = trim(gethostname() ?: '');
        $this->formatter()->info("Issuing panel SSL certificate for: {$hostname}");

        $result = $this->callAgent('ssl.panel.issue', ['hostname' => $hostname]);

        if ($result === null) {
            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json($result->toArray());

            return Command::SUCCESS;
        }

        $this->formatter()->info("Result:  {$result->get('message', 'Done')}");
        $this->formatter()->info("Issuer:  {$result->get('issuer', '-')}");
        $this->formatter()->info("From:    {$result->get('valid_from', '-')}");
        $this->formatter()->info("To:      {$result->get('valid_to', '-')}");

        return Command::SUCCESS;
    }
}
