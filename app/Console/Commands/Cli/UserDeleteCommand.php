<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use App\Models\User;
use Illuminate\Console\Command;

class UserDeleteCommand extends JabaliCommand
{
    protected $signature = 'jabali:user:delete {username} {--json} {--yes}';

    protected $description = 'Delete a user';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $username = $this->argument('username');
        $user = User::where('username', $username)->orWhere('name', $username)->first();

        if (! $user) {
            $this->formatter()->error("User not found: {$username}");

            return Command::FAILURE;
        }

        if (! $this->confirmAction("Are you sure you want to delete user '{$username}'? This cannot be undone.")) {
            return Command::SUCCESS;
        }

        // Delete system user via agent
        $result = $this->callAgent('user.delete', ['username' => $username]);

        if ($result === null) {
            return Command::FAILURE;
        }

        $user->delete();

        if ($this->option('json')) {
            $this->formatter()->json(['deleted' => $username]);

            return Command::SUCCESS;
        }

        $this->formatter()->success("User '{$username}' deleted");

        return Command::SUCCESS;
    }
}
