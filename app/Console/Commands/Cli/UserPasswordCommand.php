<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use App\Models\User;
use Illuminate\Console\Command;

class UserPasswordCommand extends JabaliCommand
{
    protected $signature = 'jabali:user:password {username} {--password=} {--json} {--yes}';

    protected $description = 'Update a user password';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $username = $this->argument('username');
        $generated = false;
        $password = $this->option('password');

        if (! $password) {
            $password = substr(str_replace(['/', '+', '='], '', base64_encode(random_bytes(16))), 0, 16);
            $generated = true;
        }

        if ($error = $this->validatePassword($password)) {
            $this->formatter()->error($error);

            return Command::FAILURE;
        }

        $user = User::where('username', $username)->orWhere('name', $username)->first();

        if (! $user) {
            $this->formatter()->error("User not found: {$username}");

            return Command::FAILURE;
        }

        // Update panel user password
        $user->password = bcrypt($password);
        $user->save();

        // Sync system user password via agent
        $this->callAgent('user.password', [
            'username' => $username,
            'password' => $password,
        ]);

        if ($this->option('json')) {
            $data = ['username' => $username, 'password_updated' => true];
            if ($generated) {
                $data['password'] = $password;
            }
            $this->formatter()->json($data);

            return Command::SUCCESS;
        }

        $this->formatter()->success("Password updated for '{$username}'");
        if ($generated) {
            $this->formatter()->info("Generated password: {$password}");
        }

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
