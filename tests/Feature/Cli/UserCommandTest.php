<?php

declare(strict_types=1);

namespace Tests\Feature\Cli;

use App\Console\Commands\Cli\UserAdminCommand;
use App\Console\Commands\Cli\UserCreateCommand;
use App\Console\Commands\Cli\UserDeleteCommand;
use App\Console\Commands\Cli\UserListCommand;
use App\Console\Commands\Cli\UserPasswordCommand;
use App\Console\Commands\Cli\UserShowCommand;
use App\Console\Commands\Cli\UserSuspendCommand;
use App\Console\Commands\Cli\UserUnsuspendCommand;
use App\Models\User;
use App\Services\Agent\AgentClientInterface;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class UserCommandTest extends TestCase
{
    use RefreshDatabase;

    protected function setUp(): void
    {
        parent::setUp();

        $kernel = $this->app[\Illuminate\Contracts\Console\Kernel::class];
        $kernel->registerCommand(new UserListCommand);
        $kernel->registerCommand(new UserCreateCommand);
        $kernel->registerCommand(new UserShowCommand);
        $kernel->registerCommand(new UserDeleteCommand);
        $kernel->registerCommand(new UserPasswordCommand);
        $kernel->registerCommand(new UserAdminCommand);
        $kernel->registerCommand(new UserSuspendCommand);
        $kernel->registerCommand(new UserUnsuspendCommand);
    }

    public function test_user_list_shows_users(): void
    {
        User::factory()->create(['username' => 'alice', 'email' => 'alice@example.com']);
        User::factory()->create(['username' => 'bob', 'email' => 'bob@example.com']);

        $this->artisan('jabali:user:list')
            ->assertExitCode(0)
            ->expectsOutputToContain('alice')
            ->expectsOutputToContain('bob');
    }

    public function test_user_list_json_output(): void
    {
        User::factory()->create(['username' => 'alice', 'email' => 'alice@example.com']);

        $this->artisan('jabali:user:list', ['--json' => true])
            ->assertExitCode(0)
            ->expectsOutputToContain('alice@example.com');
    }

    public function test_user_create_calls_agent_and_creates_record(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn(['success' => true]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:user:create', [
            'username' => 'newuser',
            '--email' => 'new@example.com',
            '--password' => 'Secret1pass',
        ])->assertExitCode(0);

        $this->assertDatabaseHas('users', [
            'username' => 'newuser',
            'email' => 'new@example.com',
        ]);
    }

    public function test_user_delete_requires_confirmation(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn(['success' => true]);
        $this->app->instance(AgentClientInterface::class, $agent);

        User::factory()->create(['username' => 'testdel', 'email' => 'del@example.com']);

        $this->artisan('jabali:user:delete', ['username' => 'testdel'])
            ->expectsConfirmation("Are you sure you want to delete user 'testdel'? This cannot be undone.", 'no')
            ->assertExitCode(0);

        $this->assertDatabaseHas('users', ['username' => 'testdel']);
    }

    public function test_user_delete_with_yes_skips_confirmation(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn(['success' => true]);
        $this->app->instance(AgentClientInterface::class, $agent);

        User::factory()->create(['username' => 'testdel2', 'email' => 'del2@example.com']);

        $this->artisan('jabali:user:delete', ['username' => 'testdel2', '--yes' => true])
            ->assertExitCode(0);

        $this->assertDatabaseMissing('users', ['username' => 'testdel2']);
    }

    public function test_user_suspend_updates_model(): void
    {
        User::factory()->create(['username' => 'tosuspend', 'email' => 'suspend@example.com']);

        $this->artisan('jabali:user:suspend', ['username' => 'tosuspend'])
            ->assertExitCode(0);

        $user = User::where('username', 'tosuspend')->first();
        $this->assertFalse((bool) $user->is_active);
    }

    public function test_user_admin_requires_confirmation(): void
    {
        User::factory()->create(['username' => 'regular', 'email' => 'regular@example.com', 'is_admin' => false]);

        $this->artisan('jabali:user:admin', ['identifier' => 'regular'])
            ->expectsConfirmation("Are you sure you want to grant admin access to 'regular'?", 'no')
            ->assertExitCode(0);

        $user = User::where('username', 'regular')->first();
        $this->assertFalse((bool) $user->is_admin);
    }
}
