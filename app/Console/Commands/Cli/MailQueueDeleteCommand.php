<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class MailQueueDeleteCommand extends JabaliCommand
{
    protected $signature = 'jabali:mail:queue-delete {id} {--json} {--yes}';

    protected $description = 'Delete a message from the mail queue';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $id = $this->argument('id');

        if (! $this->confirmAction("Delete message {$id}? This cannot be undone.")) {
            return Command::FAILURE;
        }

        $result = $this->callAgent('mail.queue_delete', ['id' => $id]);

        if ($result === null) {
            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json($result->data ?? []);

            return Command::SUCCESS;
        }

        $this->formatter()->success("Message {$id} deleted");

        return Command::SUCCESS;
    }
}
