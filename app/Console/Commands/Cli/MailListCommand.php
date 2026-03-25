<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use App\Models\Mailbox;
use Illuminate\Console\Command;

class MailListCommand extends JabaliCommand
{
    protected $signature = 'jabali:mail:list {--domain=} {--json} {--yes}';

    protected $description = 'List mailboxes';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $query = Mailbox::with('emailDomain.domain');

        if ($domain = $this->option('domain')) {
            $query->whereHas('emailDomain.domain', function ($q) use ($domain) {
                $q->where('domain', $domain);
            });
        }

        $mailboxes = $query->get();

        if ($this->option('json')) {
            $this->formatter()->json($mailboxes->map(fn (Mailbox $m) => [
                'email' => $m->local_part.'@'.$m->emailDomain->domain->domain,
                'domain' => $m->emailDomain->domain->domain,
                'quota' => $m->quota_formatted,
                'created_at' => $m->created_at?->toDateTimeString(),
            ])->toArray());

            return Command::SUCCESS;
        }

        $rows = [];
        foreach ($mailboxes as $mailbox) {
            $domainName = $mailbox->emailDomain->domain->domain ?? '-';
            $rows[] = [
                $mailbox->local_part.'@'.$domainName,
                $domainName,
                $mailbox->quota_formatted,
                $mailbox->created_at?->format('Y-m-d') ?? '-',
            ];
        }

        $this->formatter()->table(['Email', 'Domain', 'Quota', 'Created'], $rows);

        return Command::SUCCESS;
    }
}
