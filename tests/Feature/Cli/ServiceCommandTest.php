<?php

declare(strict_types=1);

namespace Tests\Feature\Cli;

use App\Console\Commands\Cli\ServiceCommand;
use App\Console\Commands\Cli\ServiceDisableCommand;
use App\Console\Commands\Cli\ServiceEnableCommand;
use App\Console\Commands\Cli\ServiceRestartCommand;
use App\Console\Commands\Cli\ServiceStartCommand;
use App\Console\Commands\Cli\ServiceStatusCommand;
use App\Console\Commands\Cli\ServiceStopCommand;
use App\Services\Agent\AgentClientInterface;
use Tests\TestCase;

class ServiceCommandTest extends TestCase
{
    protected function setUp(): void
    {
        parent::setUp();
        $kernel = $this->app[\Illuminate\Contracts\Console\Kernel::class];
        $kernel->registerCommand(new ServiceCommand);
        $kernel->registerCommand(new ServiceStatusCommand);
        $kernel->registerCommand(new ServiceStartCommand);
        $kernel->registerCommand(new ServiceStopCommand);
        $kernel->registerCommand(new ServiceRestartCommand);
        $kernel->registerCommand(new ServiceEnableCommand);
        $kernel->registerCommand(new ServiceDisableCommand);
    }

    public function test_service_list_shows_services(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn([
            'success' => true,
            'services' => [
                'nginx' => ['is_active' => true, 'is_enabled' => true],
                'mariadb' => ['is_active' => true, 'is_enabled' => true],
                'postfix' => ['is_active' => false, 'is_enabled' => false],
            ],
        ]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:service:list')
            ->assertExitCode(0)
            ->expectsOutputToContain('nginx')
            ->expectsOutputToContain('mariadb')
            ->expectsOutputToContain('postfix');
    }

    public function test_service_list_json_outputs_valid_json(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn([
            'success' => true,
            'services' => [
                'nginx' => ['is_active' => true, 'is_enabled' => true],
            ],
        ]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:service:list', ['--json' => true])
            ->assertExitCode(0);
    }

    public function test_service_status_shows_single_service(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn([
            'success' => true,
            'services' => [
                'nginx' => ['is_active' => true, 'is_enabled' => true],
            ],
        ]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:service:status', ['name' => 'nginx'])
            ->assertExitCode(0)
            ->expectsOutputToContain('nginx');
    }

    public function test_service_start_calls_agent(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn(['success' => true]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:service:start', ['name' => 'nginx'])
            ->assertExitCode(0);
    }

    public function test_service_stop_requires_confirmation(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn(['success' => true]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:service:stop', ['name' => 'nginx'])
            ->expectsConfirmation('Are you sure you want to stop nginx?', 'no')
            ->assertExitCode(0);
    }

    public function test_service_stop_with_yes_skips_confirmation(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn(['success' => true]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:service:stop', ['name' => 'nginx', '--yes' => true])
            ->assertExitCode(0);
    }

    public function test_service_restart_calls_agent(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn(['success' => true]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:service:restart', ['name' => 'nginx'])
            ->assertExitCode(0);
    }

    public function test_service_disable_requires_confirmation(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn(['success' => true]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:service:disable', ['name' => 'nginx'])
            ->expectsConfirmation('Are you sure you want to disable nginx on boot?', 'no')
            ->assertExitCode(0);
    }

    public function test_service_failed_agent_returns_exit_code_1(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn([
            'success' => false,
            'error' => 'Permission denied',
        ]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:service:start', ['name' => 'nginx'])
            ->assertExitCode(1);
    }
}
