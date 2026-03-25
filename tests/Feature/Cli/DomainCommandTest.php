<?php

declare(strict_types=1);

namespace Tests\Feature\Cli;

use App\Console\Commands\Cli\DomainCreateCommand;
use App\Console\Commands\Cli\DomainDeleteCommand;
use App\Console\Commands\Cli\DomainDisableCommand;
use App\Console\Commands\Cli\DomainEnableCommand;
use App\Console\Commands\Cli\DomainListCommand;
use App\Console\Commands\Cli\DomainShowCommand;
use App\Models\Domain;
use App\Models\User;
use App\Services\Agent\AgentClientInterface;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class DomainCommandTest extends TestCase
{
    use RefreshDatabase;

    protected function setUp(): void
    {
        parent::setUp();
        $kernel = $this->app[\Illuminate\Contracts\Console\Kernel::class];
        $kernel->registerCommand(new DomainListCommand);
        $kernel->registerCommand(new DomainCreateCommand);
        $kernel->registerCommand(new DomainShowCommand);
        $kernel->registerCommand(new DomainDeleteCommand);
        $kernel->registerCommand(new DomainEnableCommand);
        $kernel->registerCommand(new DomainDisableCommand);
    }

    public function test_domain_list_command_exists(): void
    {
        $user = User::factory()->create(['username' => 'testuser']);
        Domain::create([
            'user_id' => $user->id,
            'domain' => 'example.com',
            'is_active' => true,
        ]);

        $this->artisan('jabali:domain:list')
            ->assertExitCode(0)
            ->expectsOutputToContain('example.com');
    }

    public function test_domain_create_calls_agent(): void
    {
        $user = User::factory()->create(['username' => 'testuser']);

        $agent = $this->createMock(AgentClientInterface::class);
        $agent->expects($this->once())
            ->method('send')
            ->with('domain.create', [
                'username' => 'testuser',
                'domain' => 'newdomain.com',
            ])
            ->willReturn(['success' => true]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:domain:create', [
            'domain' => 'newdomain.com',
            '--user' => 'testuser',
        ])->assertExitCode(0);

        $this->assertDatabaseHas('domains', [
            'domain' => 'newdomain.com',
            'user_id' => $user->id,
        ]);
    }

    public function test_domain_delete_requires_confirmation(): void
    {
        $user = User::factory()->create(['username' => 'testuser']);
        Domain::create([
            'user_id' => $user->id,
            'domain' => 'delete-me.com',
            'is_active' => true,
        ]);

        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn(['success' => true]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:domain:delete', ['domain' => 'delete-me.com'])
            ->expectsConfirmation("Are you sure you want to delete domain 'delete-me.com'?", 'no')
            ->assertExitCode(0);

        $this->assertDatabaseHas('domains', ['domain' => 'delete-me.com']);
    }

    public function test_domain_enable_updates_model(): void
    {
        $user = User::factory()->create(['username' => 'testuser']);
        $domain = Domain::create([
            'user_id' => $user->id,
            'domain' => 'disabled.com',
            'is_active' => false,
        ]);

        $this->artisan('jabali:domain:enable', ['domain' => 'disabled.com'])
            ->assertExitCode(0);

        $this->assertTrue($domain->fresh()->is_active);
    }

    public function test_domain_list_json_output(): void
    {
        $user = User::factory()->create(['username' => 'testuser']);
        Domain::create([
            'user_id' => $user->id,
            'domain' => 'json-test.com',
            'is_active' => true,
        ]);

        $this->artisan('jabali:domain:list', ['--json' => true])
            ->assertExitCode(0);
    }

    public function test_domain_show_displays_details(): void
    {
        $user = User::factory()->create(['username' => 'testuser']);
        Domain::create([
            'user_id' => $user->id,
            'domain' => 'show-me.com',
            'is_active' => true,
        ]);

        $this->artisan('jabali:domain:show', ['domain' => 'show-me.com'])
            ->assertExitCode(0)
            ->expectsOutputToContain('show-me.com')
            ->expectsOutputToContain('testuser');
    }

    public function test_domain_show_not_found_returns_failure(): void
    {
        $this->artisan('jabali:domain:show', ['domain' => 'ghost.com'])
            ->assertExitCode(1);
    }

    public function test_domain_disable_updates_model(): void
    {
        $user = User::factory()->create(['username' => 'testuser']);
        $domain = Domain::create([
            'user_id' => $user->id,
            'domain' => 'to-disable.com',
            'is_active' => true,
        ]);

        $this->artisan('jabali:domain:disable', ['domain' => 'to-disable.com'])
            ->assertExitCode(0);

        $this->assertFalse($domain->fresh()->is_active);
    }

    public function test_domain_delete_with_yes_deletes_record(): void
    {
        $user = User::factory()->create(['username' => 'testuser']);
        Domain::create([
            'user_id' => $user->id,
            'domain' => 'bye.com',
            'is_active' => true,
        ]);

        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn(['success' => true]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:domain:delete', ['domain' => 'bye.com', '--yes' => true])
            ->assertExitCode(0);

        $this->assertDatabaseMissing('domains', ['domain' => 'bye.com']);
    }
}
