<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use App\Models\Mailbox;
use Illuminate\Console\Command;

class MailDeleteCommand extends JabaliCommand
{
    protected $signature = 'jabali:mail:delete {email} {--json} {--yes}';

    protected $description = 'Delete a mailbox';

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

        if (! $this->confirmAction("Are you sure you want to delete mailbox '{$email}'?")) {
            return Command::SUCCESS;
        }

        $result = $this->callAgent('email.mailbox_delete', [
            'email' => $email,
            'domain' => $domain,
        ]);

        if ($result === null) {
            return Command::FAILURE;
        }

        // Delete Eloquent record
        $mailbox = Mailbox::whereHas('emailDomain.domain', function ($q) use ($domain) {
            $q->where('domain', $domain);
        })->where('local_part', $parts[0])->first();

        $mailbox?->delete();

        if ($this->option('json')) {
            $this->formatter()->json(['deleted' => $email]);

            return Command::SUCCESS;
        }

        $this->formatter()->success("Mailbox '{$email}' deleted");

        return Command::SUCCESS;
    }
}
