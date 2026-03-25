<?php

declare(strict_types=1);

namespace Tests\Feature\Cli;

use App\Console\Commands\Cli\FirewallAllowCommand;
use App\Console\Commands\Cli\FirewallDeleteCommand;
use App\Console\Commands\Cli\FirewallDenyCommand;
use App\Console\Commands\Cli\FirewallDisableCommand;
use App\Console\Commands\Cli\FirewallEnableCommand;
use App\Console\Commands\Cli\FirewallRulesCommand;
use App\Console\Commands\Cli\FirewallStatusCommand;
use App\Services\Agent\AgentClientInterface;
use Tests\TestCase;

class FirewallCommandTest extends TestCase
{
    protected function setUp(): void
    {
        parent::setUp();
        $kernel = $this->app[\Illuminate\Contracts\Console\Kernel::class];
        $kernel->registerCommand(new FirewallStatusCommand);
        $kernel->registerCommand(new FirewallEnableCommand);
        $kernel->registerCommand(new FirewallDisableCommand);
        $kernel->registerCommand(new FirewallRulesCommand);
        $kernel->registerCommand(new FirewallAllowCommand);
        $kernel->registerCommand(new FirewallDenyCommand);
        $kernel->registerCommand(new FirewallDeleteCommand);
    }

    public function test_firewall_status_calls_agent(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn([
            'success' => true,
            'active' => true,
            'default_policy' => 'deny',
            'rules_count' => 5,
        ]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:firewall:status')
            ->assertExitCode(0)
            ->expectsOutputToContain('Status');
    }

    public function test_firewall_enable_calls_agent(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn(['success' => true]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:firewall:enable')
            ->assertExitCode(0);
    }

    public function test_firewall_disable_requires_confirmation(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn(['success' => true]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:firewall:disable')
            ->expectsConfirmation('Are you sure you want to disable the firewall?', 'no')
            ->assertExitCode(0);
    }

    public function test_firewall_allow_calls_agent(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn(['success' => true]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:firewall:allow', ['port' => '443'])
            ->assertExitCode(0);
    }

    public function test_firewall_delete_requires_confirmation(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn(['success' => true]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:firewall:delete', ['rule_number' => '3'])
            ->expectsConfirmation('Are you sure you want to delete firewall rule #3?', 'no')
            ->assertExitCode(0);
    }
}
