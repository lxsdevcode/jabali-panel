<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class BackupPasswordCommand extends JabaliCommand
{
    protected $signature = 'jabali:backup:password {--set=} {--json} {--yes}';

    protected $description = 'Show or set the backup encryption password';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $setPassword = $this->option('set');

        if ($setPassword) {
            // Set a new password
            if (! $this->confirmAction('Are you sure you want to change the backup encryption password? All backups encrypted with the old password will become unrecoverable.')) {
                return Command::SUCCESS;
            }

            $result = $this->callAgent('backup.set_password', [
                'password' => $setPassword,
            ]);

            if ($result === null) {
                return Command::FAILURE;
            }

            if ($this->option('json')) {
                $this->formatter()->json(['password_changed' => true]);

                return Command::SUCCESS;
            }

            $this->formatter()->success('Backup encryption password updated');

            return Command::SUCCESS;
        }

        // Get current password
        $result = $this->callAgent('backup.get_password');

        if ($result === null) {
            return Command::FAILURE;
        }

        $password = $result->get('password');

        if ($this->option('json')) {
            $this->formatter()->json(['password' => $password]);

            return Command::SUCCESS;
        }

        $this->formatter()->line('Backup Encryption Password:');
        $this->formatter()->line("<fg=yellow>{$password}</>");

        return Command::SUCCESS;
    }
}
