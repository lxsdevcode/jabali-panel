<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use App\Models\EmailDomain;
use App\Models\Mailbox;
use Illuminate\Console\Command;

class MailCreateCommand extends JabaliCommand
{
    protected $signature = 'jabali:mail:create {email} {--password=} {--quota=} {--json} {--yes}';

    protected $description = 'Create a mailbox';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $email = $this->argument('email');
        $parts = explode('@', $email);

        if (count($parts) !== 2) {
            $this->formatter()->error('Invalid email address format');

            return Command::FAILURE;
        }

        [$localPart, $domain] = $parts;

        $params = [
            'email' => $email,
            'domain' => $domain,
        ];

        if ($password = $this->option('password')) {
            $params['password'] = $password;
        }

        if ($quota = $this->option('quota')) {
            $params['quota'] = $quota;
        }

        $result = $this->callAgent('email.mailbox_create', $params);

        if ($result === null) {
            return Command::FAILURE;
        }

        // Create Eloquent record
        $emailDomain = EmailDomain::whereHas('domain', function ($q) use ($domain) {
            $q->where('domain', $domain);
        })->first();

        if ($emailDomain) {
            Mailbox::create([
                'email_domain_id' => $emailDomain->id,
                'local_part' => $localPart,
                'is_active' => true,
            ]);
        }

        if ($this->option('json')) {
            $this->formatter()->json($result->toArray());

            return Command::SUCCESS;
        }

        $this->formatter()->success("Mailbox '{$email}' created");

        return Command::SUCCESS;
    }
}
