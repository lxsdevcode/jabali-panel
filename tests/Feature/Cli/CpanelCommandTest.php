<?php

declare(strict_types=1);

namespace Tests\Feature\Cli;

use App\Console\Commands\Cli\CpanelAnalyzeCommand;
use App\Console\Commands\Cli\CpanelFixPermissionsCommand;
use App\Console\Commands\Cli\CpanelRestoreCommand;
use App\Services\Agent\AgentClientInterface;
use Tests\TestCase;

class CpanelCommandTest extends TestCase
{
    protected function setUp(): void
    {
        parent::setUp();
        $kernel = $this->app[\Illuminate\Contracts\Console\Kernel::class];
        $kernel->registerCommand(new CpanelAnalyzeCommand);
        $kernel->registerCommand(new CpanelRestoreCommand);
        $kernel->registerCommand(new CpanelFixPermissionsCommand);
    }

    public function test_cpanel_analyze_calls_agent(): void
    {
        $tmpFile = tempnam(sys_get_temp_dir(), 'cpanel_test_');

        $agent = $this->createMock(AgentClientInterface::class);
        $agent->expects($this->once())
            ->method('send')
            ->with('cpanel.analyze_backup', ['path' => $tmpFile])
            ->willReturn([
                'success' => true,
                'user' => 'testuser',
                'domains' => ['example.com'],
            ]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:cpanel:analyze', ['path' => $tmpFile])
            ->assertExitCode(0)
            ->expectsOutputToContain('testuser');

        @unlink($tmpFile);
    }

    public function test_cpanel_restore_requires_confirmation(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn(['success' => true]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:cpanel:restore', ['path' => '/tmp/backup.tar.gz', '--user' => 'testuser'])
            ->expectsConfirmation("Are you sure you want to restore cPanel backup '/tmp/backup.tar.gz' for user 'testuser'?", 'no')
            ->assertExitCode(0);
    }

    public function test_cpanel_fix_permissions_calls_agent(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->expects($this->once())
            ->method('send')
            ->with('cpanel.fix_permissions', ['username' => 'testuser'])
            ->willReturn(['success' => true]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:cpanel:fix-permissions', ['username' => 'testuser'])
            ->assertExitCode(0);
    }
}
