<?php

declare(strict_types=1);

namespace Tests\Unit\Cli;

use App\Console\Cli\CliRouter;
use PHPUnit\Framework\TestCase;

class CliRouterTest extends TestCase
{
    public function test_noun_verb_maps_to_artisan_command(): void
    {
        $router = new CliRouter;

        $result = $router->resolve(['user', 'create']);

        $this->assertSame('jabali:user:create', $result->command);
        $this->assertEmpty($result->arguments);
    }

    public function test_noun_verb_with_flags_passes_arguments(): void
    {
        $router = new CliRouter;

        $result = $router->resolve(['user', 'create', '--name=Alice', '--email=alice@example.com']);

        $this->assertSame('jabali:user:create', $result->command);
        $this->assertSame('Alice', $result->arguments['--name']);
        $this->assertSame('alice@example.com', $result->arguments['--email']);
    }

    public function test_help_flag_returns_help_command(): void
    {
        $router = new CliRouter;

        $result = $router->resolve(['--help']);

        $this->assertSame('jabali:help', $result->command);
    }

    public function test_empty_args_returns_help_command(): void
    {
        $router = new CliRouter;

        $result = $router->resolve([]);

        $this->assertSame('jabali:help', $result->command);
    }

    public function test_noun_only_returns_exit_code_2(): void
    {
        $router = new CliRouter;

        $result = $router->resolve(['user']);

        $this->assertSame(2, $result->exitCode);
    }

    public function test_space_separated_flag_value(): void
    {
        $router = new CliRouter;

        $result = $router->resolve(['user', 'create', '--name', 'Alice']);

        $this->assertSame('Alice', $result->arguments['--name']);
    }
}
