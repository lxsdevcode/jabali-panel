<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use App\Models\User;
use Illuminate\Console\Command;

class UserCreateCommand extends JabaliCommand
{
    protected $signature = 'jabali:user:create {username} {--email=} {--password=} {--admin} {--force} {--json} {--yes}';

    protected $description = 'Create a new user';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $username = $this->argument('username');
        $email = $this->option('email');
        $password = $this->option('password');

        if (! $email) {
            $this->formatter()->error('The --email option is required');

            return Command::FAILURE;
        }

        if (! $password) {
            $this->formatter()->error('The --password option is required');

            return Command::FAILURE;
        }

        if ($error = $this->validatePassword($password)) {
            $this->formatter()->error($error);

            return Command::FAILURE;
        }

        // Create system user via agent
        $result = $this->callAgent('user.create', [
            'username' => $username,
            'password' => $password,
        ]);

        if ($result === null) {
            return Command::FAILURE;
        }

        // Check for existing panel user
        $panelUser = User::where('username', $username)
            ->orWhere('name', $username)
            ->orWhere('email', $email)
            ->first();

        if ($panelUser) {
            if (! $this->option('force')) {
                $this->formatter()->error("Panel user already exists for '{$username}' (ID: {$panelUser->id}). Use --force to update.");

                return Command::FAILURE;
            }

            $panelUser->name = $panelUser->name ?: $username;
            $panelUser->username = $panelUser->username ?: $username;
            $panelUser->email = $email;
            $panelUser->password = bcrypt($password);
            $panelUser->save();

            if ($this->option('json')) {
                $this->formatter()->json([
                    'id' => $panelUser->id,
                    'username' => $panelUser->username,
                    'email' => $panelUser->email,
                    'updated' => true,
                ]);

                return Command::SUCCESS;
            }

            $this->formatter()->success("User '{$username}' updated successfully (ID: {$panelUser->id})");

            return Command::SUCCESS;
        }

        // First user auto-gets admin role
        $isFirstUser = User::count() === 0;
        $isAdmin = $isFirstUser || $this->option('admin');

        $user = User::create([
            'name' => $username,
            'username' => $username,
            'email' => $email,
            'password' => bcrypt($password),
            'is_admin' => $isAdmin,
        ]);

        if ($this->option('json')) {
            $this->formatter()->json([
                'id' => $user->id,
                'username' => $user->username,
                'email' => $user->email,
                'is_admin' => $user->is_admin,
                'created' => true,
            ]);

            return Command::SUCCESS;
        }

        $this->formatter()->success("User '{$username}' created successfully (ID: {$user->id})");

        return Command::SUCCESS;
    }

    private function validatePassword(string $password): ?string
    {
        if (strlen($password) < 8) {
            return 'Password must be at least 8 characters';
        }
        if (! preg_match('/[a-z]/', $password)) {
            return 'Password must contain a lowercase letter';
        }
        if (! preg_match('/[A-Z]/', $password)) {
            return 'Password must contain an uppercase letter';
        }
        if (! preg_match('/[0-9]/', $password)) {
            return 'Password must contain a number';
        }

        return null;
    }
}
