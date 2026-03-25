<?php

declare(strict_types=1);

namespace Tests\Feature\Cli;

use App\Console\Commands\Cli\BackupCreateCommand;
use App\Console\Commands\Cli\BackupDeleteCommand;
use App\Console\Commands\Cli\BackupInfoCommand;
use App\Console\Commands\Cli\BackupListCommand;
use App\Console\Commands\Cli\BackupRestoreCommand;
use App\Services\Agent\AgentClientInterface;
use Tests\TestCase;

class BackupCommandTest extends TestCase
{
    protected function setUp(): void
    {
        parent::setUp();
        $kernel = $this->app[\Illuminate\Contracts\Console\Kernel::class];
        $kernel->registerCommand(new BackupListCommand);
        $kernel->registerCommand(new BackupCreateCommand);
        $kernel->registerCommand(new BackupRestoreCommand);
        $kernel->registerCommand(new BackupDeleteCommand);
        $kernel->registerCommand(new BackupInfoCommand);
    }

    public function test_backup_list_calls_agent(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn([
            'success' => true,
            'backups' => [
                [
                    'id' => 'backup-001',
                    'user' => 'testuser',
                    'date' => '2026-03-25',
                    'size' => '1.2 GB',
                    'type' => 'full',
                ],
            ],
        ]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:backup:list')
            ->expectsOutputToContain('backup-001')
            ->assertExitCode(0);
    }

    public function test_backup_create_calls_agent(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->expects($this->once())
            ->method('send')
            ->with('backup.create', ['username' => 'testuser'])
            ->willReturn(['success' => true]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:backup:create', ['username' => 'testuser'])
            ->assertExitCode(0);
    }

    public function test_backup_restore_requires_confirmation(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn(['success' => true]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:backup:restore', ['path' => '/tmp/backup.tar.gz'])
            ->expectsConfirmation("Are you sure you want to restore backup '/tmp/backup.tar.gz'?", 'no')
            ->assertExitCode(0);
    }

    public function test_backup_delete_requires_confirmation(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn(['success' => true]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:backup:delete', ['path' => '/tmp/backup.tar.gz'])
            ->expectsConfirmation("Are you sure you want to delete backup '/tmp/backup.tar.gz'?", 'no')
            ->assertExitCode(0);
    }
}
