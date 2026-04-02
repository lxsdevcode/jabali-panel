<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class MailQueueCommand extends JabaliCommand
{
    protected $signature = 'jabali:mail:queue {--json} {--yes}';

    protected $description = 'Show mail queue';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $result = $this->callAgent('mail.queue_list', []);

        if ($result === null) {
            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json($result->data ?? []);

            return Command::SUCCESS;
        }

        $rows = [];
        $messages = $result->data ?? [];

        if (is_array($messages)) {
            foreach ($messages as $msg) {
                $rows[] = [
                    $msg['id'] ?? '-',
                    $msg['from'] ?? '-',
                    $msg['to'] ?? '-',
                    $msg['status'] ?? '-',
                    $msg['size'] ?? '-',
                    $msg['created_at'] ?? '-',
                ];
            }
        }

        if (empty($rows)) {
            $this->formatter()->info('No messages in queue');

            return Command::SUCCESS;
        }

        $this->formatter()->table(['ID', 'From', 'To', 'Status', 'Size', 'Date'], $rows);

        return Command::SUCCESS;
    }
}
