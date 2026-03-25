<?php

declare(strict_types=1);

namespace Tests\Feature\Cli;

use App\Console\Commands\Cli\WpDeleteCommand;
use App\Console\Commands\Cli\WpImportCommand;
use App\Console\Commands\Cli\WpInstallCommand;
use App\Console\Commands\Cli\WpListCommand;
use App\Console\Commands\Cli\WpScanCommand;
use App\Console\Commands\Cli\WpUpdateCommand;
use App\Services\Agent\AgentClientInterface;
use Tests\TestCase;

class WpCommandTest extends TestCase
{
    protected function setUp(): void
    {
        parent::setUp();
        $kernel = $this->app[\Illuminate\Contracts\Console\Kernel::class];
        $kernel->registerCommand(new WpListCommand);
        $kernel->registerCommand(new WpInstallCommand);
        $kernel->registerCommand(new WpScanCommand);
        $kernel->registerCommand(new WpDeleteCommand);
        $kernel->registerCommand(new WpUpdateCommand);
        $kernel->registerCommand(new WpImportCommand);
    }

    public function test_wp_list_shows_sites(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn([
            'success' => true,
            'sites' => [
                ['site' => 'example.com', 'path' => '/home/user/public_html', 'version' => '6.4', 'status' => 'active', 'updates' => false],
                ['site' => 'blog.test', 'path' => '/home/user/blog', 'version' => '6.3', 'status' => 'active', 'updates' => true],
            ],
        ]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:wp:list')
            ->assertExitCode(0)
            ->expectsOutputToContain('example.com')
            ->expectsOutputToContain('blog.test');
    }

    public function test_wp_install_calls_agent(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->expects($this->once())
            ->method('send')
            ->with('wp.install', $this->callback(function (array $params) {
                return $params['domain'] === 'newsite.com';
            }))
            ->willReturn(['success' => true]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:wp:install', ['domain' => 'newsite.com'])
            ->assertExitCode(0);
    }

    public function test_wp_delete_requires_confirmation(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn(['success' => true]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:wp:delete', ['domain' => 'delete-me.com'])
            ->expectsConfirmation("Are you sure you want to delete WordPress on 'delete-me.com'?", 'no')
            ->assertExitCode(0);
    }

    public function test_wp_scan_calls_agent(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->expects($this->once())
            ->method('send')
            ->with('wp.scan', ['domain' => 'scanme.com'])
            ->willReturn(['success' => true]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:wp:scan', ['domain' => 'scanme.com'])
            ->assertExitCode(0);
    }

    public function test_wp_agent_failure_returns_exit_1(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn([
            'success' => false,
            'error' => 'WordPress not found',
        ]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:wp:list')
            ->assertExitCode(1);
    }
}
