<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use App\Models\User;
use Illuminate\Console\Command;

class UserSuspendCommand extends JabaliCommand
{
    protected $signature = 'jabali:user:suspend {username} {--json} {--yes}';

    protected $description = 'Suspend a user';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $username = $this->argument('username');
        $user = User::where('username', $username)->orWhere('name', $username)->first();

        if (! $user) {
            $this->formatter()->error("User not found: {$username}");

            return Command::FAILURE;
        }

        $user->is_active = false;
        $user->save();

        if ($this->option('json')) {
            $this->formatter()->json([
                'username' => $username,
                'is_active' => false,
            ]);

            return Command::SUCCESS;
        }

        $this->formatter()->success("User '{$username}' suspended");

        return Command::SUCCESS;
    }
}
