<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class MailQuotaCommand extends JabaliCommand
{
    protected $signature = 'jabali:mail:quota {email} {--size=} {--json} {--yes}';

    protected $description = 'Set a mailbox quota';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $email = $this->argument('email');
        $parts = explode('@', $email);

        if (count($parts) !== 2) {
            $this->formatter()->error('Invalid email address format');

            return Command::FAILURE;
        }

        [, $domain] = $parts;

        $params = [
            'email' => $email,
            'domain' => $domain,
        ];

        if ($size = $this->option('size')) {
            $params['quota'] = $size;
        }

        $result = $this->callAgent('email.mailbox_set_quota', $params);

        if ($result === null) {
            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json($result->toArray());

            return Command::SUCCESS;
        }

        $this->formatter()->success("Quota updated for '{$email}'");

        return Command::SUCCESS;
    }
}
