<?php

declare(strict_types=1);

namespace App\Console\Cli;

class ResolvedRoute
{
    public function __construct(
        public readonly string $command,
        public readonly array $arguments = [],
        public readonly int $exitCode = 0,
    ) {}
}

class CliRouter
{
    public function resolve(array $args): ResolvedRoute
    {
        $noun = $args[0] ?? '';

        if ($noun === '' || $noun === '--help' || $noun === '-h') {
            return new ResolvedRoute('jabali:help');
        }

        $verb = $args[1] ?? '';

        if ($verb === '' || str_starts_with($verb, '-')) {
            return new ResolvedRoute('', [], 2);
        }

        $arguments = $this->parseArguments(array_slice($args, 2));

        return new ResolvedRoute(
            command: "jabali:{$noun}:{$verb}",
            arguments: $arguments,
        );
    }

    private function parseArguments(array $args): array
    {
        $parsed = [];

        for ($i = 0; $i < count($args); $i++) {
            $arg = $args[$i];

            if (str_starts_with($arg, '--')) {
                if (str_contains($arg, '=')) {
                    [$key, $value] = explode('=', $arg, 2);
                    $parsed[$key] = $value;
                } elseif (isset($args[$i + 1]) && ! str_starts_with($args[$i + 1], '-')) {
                    $parsed[$arg] = $args[++$i];
                } else {
                    $parsed[$arg] = true;
                }
            }
        }

        return $parsed;
    }
}
