<?php

declare(strict_types=1);

namespace App\Console\Cli;

use App\Services\Agent\AgentClientInterface;
use App\Services\Agent\AgentResult;
use Illuminate\Console\Command;
use Symfony\Component\Console\Output\ConsoleOutput;

abstract class JabaliCommand extends Command
{
    private ?OutputFormatter $outputFormatter = null;

    protected function setupFormatter(): void
    {
        $mode = 'normal';
        if ($this->option('json')) {
            $mode = 'json';
        } elseif ($this->option('quiet')) {
            $mode = 'quiet';
        }

        $consoleOutput = $this->getOutput()->getOutput();
        $errorOutput = ($consoleOutput instanceof ConsoleOutput)
            ? $consoleOutput->getErrorOutput()
            : $consoleOutput;

        $this->outputFormatter = new OutputFormatter($consoleOutput, $mode, $errorOutput);
    }

    protected function formatter(): OutputFormatter
    {
        if ($this->outputFormatter === null) {
            $this->setupFormatter();
        }

        return $this->outputFormatter;
    }

    protected function confirmAction(string $message): bool
    {
        if ($this->option('yes')) {
            return true;
        }

        return $this->confirm($message);
    }

    protected function callAgent(string $action, array $params = []): ?AgentResult
    {
        $agent = app(AgentClientInterface::class);
        $response = $agent->send($action, $params);
        $result = AgentResult::fromResponse($response);

        if ($result->failed()) {
            $this->formatter()->error($result->error ?? 'Agent call failed');
            return null;
        }

        return $result;
    }
}
