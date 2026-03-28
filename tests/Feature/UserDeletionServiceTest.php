<?php

declare(strict_types=1);

namespace Tests\Feature;

use App\Models\MysqlCredential;
use App\Models\User;
use App\Services\Agent\AgentClient;
use App\Services\UserDeletionService;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Mockery;
use Tests\TestCase;

class UserDeletionServiceTest extends TestCase
{
    use RefreshDatabase;

    public function test_delete_removes_user_and_cleans_up_forwarders(): void
    {
        $user = User::factory()->create(['username' => 'testuser']);

        $agent = Mockery::mock(AgentClient::class);
        $agent->shouldReceive('send')->zeroOrMoreTimes()->andReturn([]);

        $service = new UserDeletionService($agent);
        $service->delete($user);

        $this->assertDatabaseMissing('users', ['id' => $user->id]);
    }

    public function test_delete_prevents_primary_admin_deletion(): void
    {
        $admin = User::factory()->create(['is_admin' => true]);
        // Force ID to 1 for primary admin check
        $admin->id = 1;
        $admin->save();

        $agent = Mockery::mock(AgentClient::class);
        $service = new UserDeletionService($agent);

        $this->expectException(\RuntimeException::class);
        $this->expectExceptionMessage('Primary admin');
        $service->delete($admin);
    }

    public function test_delete_cleans_up_mysql_credentials(): void
    {
        $user = User::factory()->create(['username' => 'dbuser']);
        MysqlCredential::create([
            'user_id' => $user->id,
            'mysql_username' => 'dbuser_admin',
            'mysql_password_encrypted' => 'encrypted_pass',
        ]);

        $agent = Mockery::mock(AgentClient::class);
        $agent->shouldReceive('send')->zeroOrMoreTimes()->andReturn([]);

        $service = new UserDeletionService($agent);
        $service->delete($user);

        $this->assertDatabaseMissing('mysql_credentials', ['user_id' => $user->id]);
    }
}
