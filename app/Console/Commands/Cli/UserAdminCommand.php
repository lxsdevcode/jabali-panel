<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use App\Models\User;
use Illuminate\Console\Command;

class UserAdminCommand extends JabaliCommand
{
    protected $signature = 'jabali:user:admin {identifier} {--revoke} {--force} {--json} {--yes}';

    protected $description = 'Grant or revoke admin role';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $identifier = $this->argument('identifier');
        $revoke = $this->option('revoke');
        $makeAdmin = ! $revoke;

        $query = User::query();
        if (ctype_digit((string) $identifier)) {
            $query->where('id', (int) $identifier);
        } elseif (str_contains((string) $identifier, '@')) {
            $query->where('email', (string) $identifier);
        } else {
            $query->where('username', (string) $identifier)
                ->orWhere('name', (string) $identifier);
        }

        $user = $query->first();

        if (! $user) {
            $this->formatter()->error("User not found: {$identifier}");

            return Command::FAILURE;
        }

        $label = $user->username ?? $user->name ?? $user->email ?? ('User #'.$user->id);

        if ((bool) $user->is_admin === (bool) $makeAdmin) {
            $this->formatter()->info(
                $makeAdmin
                    ? "User '{$label}' is already an admin"
                    : "User '{$label}' is already a regular user"
            );

            return Command::SUCCESS;
        }

        if ($revoke) {
            $adminCount = User::where('is_admin', true)->count();
            if ($adminCount <= 1 && ! $this->option('force')) {
                $this->formatter()->error('Refusing to revoke admin from the last admin user. Use --force to override.');

                return Command::FAILURE;
            }
        }

        $action = $makeAdmin ? 'grant admin access to' : 'revoke admin access from';
        if (! $this->confirmAction("Are you sure you want to {$action} '{$label}'?")) {
            return Command::SUCCESS;
        }

        $user->is_admin = $makeAdmin;
        $user->save();

        $message = $makeAdmin
            ? "User '{$label}' is now an admin"
            : "User '{$label}' is no longer an admin";

        if ($this->option('json')) {
            $this->formatter()->json([
                'username' => $label,
                'is_admin' => $makeAdmin,
            ]);

            return Command::SUCCESS;
        }

        $this->formatter()->success($message);

        return Command::SUCCESS;
    }
}
