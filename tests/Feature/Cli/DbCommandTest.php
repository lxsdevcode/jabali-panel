<?php

declare(strict_types=1);

namespace Tests\Feature\Cli;

use App\Console\Commands\Cli\DbCreateCommand;
use App\Console\Commands\Cli\DbDeleteCommand;
use App\Console\Commands\Cli\DbListCommand;
use App\Console\Commands\Cli\DbUserCreateCommand;
use App\Console\Commands\Cli\DbUserDeleteCommand;
use App\Console\Commands\Cli\DbUsersCommand;
use App\Services\Agent\AgentClientInterface;
use Tests\TestCase;

class DbCommandTest extends TestCase
{
    protected function setUp(): void
    {
        parent::setUp();
        $kernel = $this->app[\Illuminate\Contracts\Console\Kernel::class];
        $kernel->registerCommand(new DbListCommand);
        $kernel->registerCommand(new DbCreateCommand);
        $kernel->registerCommand(new DbDeleteCommand);
        $kernel->registerCommand(new DbUsersCommand);
        $kernel->registerCommand(new DbUserCreateCommand);
        $kernel->registerCommand(new DbUserDeleteCommand);
    }

    public function test_db_list_shows_databases(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn([
            'success' => true,
            'databases' => [
                ['name' => 'app_db', 'size' => '50 MB', 'tables' => '24', 'user' => 'app_user'],
            ],
        ]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:db:list')
            ->assertExitCode(0)
            ->expectsOutputToContain('app_db');
    }

    public function test_db_create_calls_agent(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->expects($this->once())
            ->method('send')
            ->with('mysql.create_database', $this->callback(function (array $params) {
                return $params['name'] === 'new_db';
            }))
            ->willReturn(['success' => true]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:db:create', ['name' => 'new_db'])
            ->assertExitCode(0);
    }

    public function test_db_delete_requires_confirmation(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn(['success' => true]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:db:delete', ['name' => 'old_db'])
            ->expectsConfirmation("Are you sure you want to delete database 'old_db'?", 'no')
            ->assertExitCode(0);
    }

    public function test_db_delete_with_yes_skips_confirmation(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->expects($this->once())
            ->method('send')
            ->with('mysql.delete_database', ['name' => 'old_db'])
            ->willReturn(['success' => true]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:db:delete', ['name' => 'old_db', '--yes' => true])
            ->assertExitCode(0);
    }

    public function test_db_user_create_calls_agent(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->expects($this->once())
            ->method('send')
            ->with('mysql.create_user', $this->callback(function (array $params) {
                return $params['name'] === 'new_user' && $params['password'] === 'secret123';
            }))
            ->willReturn(['success' => true]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:db:user-create', ['name' => 'new_user', '--password' => 'secret123'])
            ->assertExitCode(0);
    }
}
