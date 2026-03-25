<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class MailPasswordCommand extends JabaliCommand
{
    protected $signature = 'jabali:mail:password {email} {--password=} {--json} {--yes}';

    protected $description = 'Change a mailbox password';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $email = $this->argument('email');
        $password = $this->option('password');
        $parts = explode('@', $email);

        if (count($parts) !== 2) {
            $this->formatter()->error('Invalid email address format');

            return Command::FAILURE;
        }

        [, $domain] = $parts;

        if (! $password) {
            $this->formatter()->error('The --password option is required');

            return Command::FAILURE;
        }

        $result = $this->callAgent('email.mailbox_change_password', [
            'email' => $email,
            'password' => $password,
            'domain' => $domain,
        ]);

        if ($result === null) {
            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json([
                'email' => $email,
                'password_updated' => true,
            ]);

            return Command::SUCCESS;
        }

        $this->formatter()->success("Password updated for '{$email}'");

        return Command::SUCCESS;
    }
}
