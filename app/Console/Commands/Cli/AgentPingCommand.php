<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;

class AgentPingCommand extends JabaliCommand
{
    protected $signature = 'jabali:agent:ping {--json} {--yes}';

    protected $description = 'Ping the agent';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $start = microtime(true);
        $result = $this->callAgent('ping');
        $elapsed = round((microtime(true) - $start) * 1000, 1);

        if ($result === null) {
            return Command::FAILURE;
        }

        if ($this->option('json')) {
            $this->formatter()->json([
                'status' => 'ok',
                'response_time_ms' => $elapsed,
            ]);

            return Command::SUCCESS;
        }

        $this->formatter()->success("Agent responded in {$elapsed}ms");

        return Command::SUCCESS;
    }
}
