<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use App\Models\User;
use Illuminate\Console\Command;

class UserUnsuspendCommand extends JabaliCommand
{
    protected $signature = 'jabali:user:unsuspend {username} {--json} {--yes}';

    protected $description = 'Unsuspend a user';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $username = $this->argument('username');
        $user = User::where('username', $username)->orWhere('name', $username)->first();

        if (! $user) {
            $this->formatter()->error("User not found: {$username}");

            return Command::FAILURE;
        }

        $user->is_active = true;
        $user->save();

        if ($this->option('json')) {
            $this->formatter()->json([
                'username' => $username,
                'is_active' => true,
            ]);

            return Command::SUCCESS;
        }

        $this->formatter()->success("User '{$username}' unsuspended");

        return Command::SUCCESS;
    }
}
