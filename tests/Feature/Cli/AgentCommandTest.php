<?php

declare(strict_types=1);

namespace Tests\Feature\Cli;

use App\Console\Commands\Cli\AgentPingCommand;
use App\Console\Commands\Cli\AgentStatusCommand;
use App\Services\Agent\AgentClientInterface;
use Tests\TestCase;

class AgentCommandTest extends TestCase
{
    protected function setUp(): void
    {
        parent::setUp();
        $kernel = $this->app[\Illuminate\Contracts\Console\Kernel::class];
        $kernel->registerCommand(new AgentPingCommand);
        $kernel->registerCommand(new AgentStatusCommand);
    }

    public function test_agent_ping_calls_agent(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn(['success' => true]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:agent:ping')
            ->assertExitCode(0);
    }

    public function test_agent_status_shows_info(): void
    {
        $this->artisan('jabali:agent:status')
            ->assertExitCode(0)
            ->expectsOutputToContain('Status');
    }
}
