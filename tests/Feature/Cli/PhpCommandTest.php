<?php

declare(strict_types=1);

namespace Tests\Feature\Cli;

use App\Console\Commands\Cli\PhpDefaultCommand;
use App\Console\Commands\Cli\PhpInstallCommand;
use App\Console\Commands\Cli\PhpListCommand;
use App\Console\Commands\Cli\PhpUninstallCommand;
use App\Services\Agent\AgentClientInterface;
use Tests\TestCase;

class PhpCommandTest extends TestCase
{
    protected function setUp(): void
    {
        parent::setUp();
        $kernel = $this->app[\Illuminate\Contracts\Console\Kernel::class];
        $kernel->registerCommand(new PhpListCommand);
        $kernel->registerCommand(new PhpInstallCommand);
        $kernel->registerCommand(new PhpUninstallCommand);
        $kernel->registerCommand(new PhpDefaultCommand);
    }

    public function test_php_list_calls_agent(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn([
            'success' => true,
            'versions' => [
                ['version' => '8.2', 'active' => true, 'default' => true],
                ['version' => '8.3', 'active' => true, 'default' => false],
            ],
        ]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:php:list')
            ->assertExitCode(0)
            ->expectsOutputToContain('8.2')
            ->expectsOutputToContain('8.3');
    }

    public function test_php_install_calls_agent(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn(['success' => true]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:php:install', ['version' => '8.3'])
            ->assertExitCode(0);
    }

    public function test_php_uninstall_requires_confirmation(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn(['success' => true]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:php:uninstall', ['version' => '8.1'])
            ->expectsConfirmation('Are you sure you want to uninstall PHP 8.1?', 'no')
            ->assertExitCode(0);
    }

    public function test_php_default_calls_agent(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn(['success' => true]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:php:default', ['version' => '8.3'])
            ->assertExitCode(0);
    }
}
