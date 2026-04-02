<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class DbTuneCommand extends JabaliCommand
{
    protected $signature = 'jabali:db:tune {variable} {value} {--persist} {--json} {--yes}';

    protected $description = 'Set a MySQL tuning variable';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $variable = $this->argument('variable');
        $value = $this->argument('value');
        $persist = $this->option('persist');

        // Set the variable at runtime
        $result = $this->callAgent('database.set_global', [
            'variable' => $variable,
            'value' => $value,
        ]);

        if ($result === null) {
            return Command::FAILURE;
        }

        // Persist to configuration if requested
        if ($persist) {
            $persistResult = $this->callAgent('database.persist_tuning', [
                'settings' => [$variable => $value],
            ]);

            if ($persistResult === null) {
                $this->formatter()->warning('Variable set at runtime but failed to persist');

                if ($this->option('json')) {
                    $this->formatter()->json([
                        'variable' => $variable,
                        'value' => $value,
                        'runtime' => true,
                        'persisted' => false,
                    ]);
                }

                return Command::FAILURE;
            }
        }

        if ($this->option('json')) {
            $this->formatter()->json([
                'variable' => $variable,
                'value' => $value,
                'runtime' => true,
                'persisted' => $persist,
            ]);

            return Command::SUCCESS;
        }

        $message = $persist
            ? "MySQL variable '{$variable}' set to '{$value}' and persisted"
            : "MySQL variable '{$variable}' set to '{$value}' (runtime only)";

        $this->formatter()->success($message);

        return Command::SUCCESS;
    }
}
