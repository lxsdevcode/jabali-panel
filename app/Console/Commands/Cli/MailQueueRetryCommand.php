<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class MailQueueRetryCommand extends JabaliCommand
{
    protected $signature = 'jabali:mail:queue-retry {id?} {--all} {--json} {--yes}';

    protected $description = 'Retry queued mail delivery';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $id = $this->argument('id');
        $all = $this->option('all');

        if (! $id && ! $all) {
            $this->formatter()->error('Provide either a message ID or use --all flag');

            return Command::FAILURE;
        }

        $params = [];

        if ($all) {
            if (! $this->confirmAction('Retry all messages in queue?')) {
                return Command::FAILURE;
            }

            $params = ['all' => true];
        } else {
            if (! $this->confirmAction("Retry message {$id}?")) {
                return Command::FAILURE;
            }

            $params = ['id' => $id];
        }

        $result = $this->callAgent('mail.queue_retry', $params);

        if ($result === null) {
            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json($result->data ?? []);

            return Command::SUCCESS;
        }

        $retried = $result->data['retried'] ?? 0;
        $this->formatter()->success("{$retried} message(s) queued for retry");

        return Command::SUCCESS;
    }
}
