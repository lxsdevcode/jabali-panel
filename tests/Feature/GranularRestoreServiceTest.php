<?php

declare(strict_types=1);

namespace Tests\Feature;

use App\Models\BackupDestination;
use App\Models\BackupRestore;
use App\Models\User;
use App\Models\UserRemoteBackup;
use App\Services\Agent\AgentClientInterface;
use App\Services\Backup\GranularRestoreService;
use Carbon\Carbon;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Mockery;
use RuntimeException;
use Tests\TestCase;

class GranularRestoreServiceTest extends TestCase
{
    use RefreshDatabase;

    public function test_list_snapshots_returns_snapshots_sorted_newest_first(): void
    {
        $user = User::factory()->create();

        $destination = BackupDestination::create([
            'user_id' => $user->id,
            'name' => 'Test SFTP',
            'type' => 'sftp',
            'config' => ['host' => '10.0.0.1', 'port' => 22, 'username' => 'backup'],
            'is_active' => true,
        ]);

        $old = UserRemoteBackup::create([
            'user_id' => $user->id,
            'destination_id' => $destination->id,
            'backup_name' => '2026-03-20_100001',
            'backup_path' => '2026-03-20_100001/'.$user->username,
            'backup_type' => 'incremental',
            'backup_date' => Carbon::parse('2026-03-20 10:00:01'),
        ]);

        $newest = UserRemoteBackup::create([
            'user_id' => $user->id,
            'destination_id' => $destination->id,
            'backup_name' => '2026-03-25_170002',
            'backup_path' => '2026-03-25_170002/'.$user->username,
            'backup_type' => 'incremental',
            'backup_date' => Carbon::parse('2026-03-25 17:00:02'),
        ]);

        $middle = UserRemoteBackup::create([
            'user_id' => $user->id,
            'destination_id' => $destination->id,
            'backup_name' => '2026-03-22_140001',
            'backup_path' => '2026-03-22_140001/'.$user->username,
            'backup_type' => 'incremental',
            'backup_date' => Carbon::parse('2026-03-22 14:00:01'),
        ]);

        $service = app(GranularRestoreService::class);
        $snapshots = $service->listSnapshots($user);

        $this->assertCount(3, $snapshots);
        $this->assertEquals($newest->id, $snapshots->first()->id);
        $this->assertEquals($old->id, $snapshots->last()->id);
    }

    public function test_full_account_restore_creates_record_and_calls_agent(): void
    {
        $user = User::factory()->create(['username' => 'testuser']);

        $destination = BackupDestination::create([
            'user_id' => $user->id,
            'name' => 'Test SFTP',
            'type' => 'sftp',
            'config' => ['host' => '10.0.0.1', 'port' => 22, 'username' => 'backup'],
            'is_active' => true,
        ]);

        $snapshot = UserRemoteBackup::create([
            'user_id' => $user->id,
            'destination_id' => $destination->id,
            'backup_name' => '2026-03-25_170002',
            'backup_path' => '2026-03-25_170002/testuser',
            'backup_type' => 'incremental',
            'backup_date' => Carbon::parse('2026-03-25 17:00:02'),
        ]);

        $mockAgent = Mockery::mock(AgentClientInterface::class);
        $mockAgent->shouldReceive('send')->once()
            ->with('backup.restore', Mockery::on(function (array $params) {
                return $params['username'] === 'testuser'
                    && $params['restore_files'] === true
                    && $params['restore_databases'] === true
                    && $params['restore_mailboxes'] === true
                    && $params['restore_dns'] === true
                    && $params['restore_ssl'] === true
                    && $params['restore_mysql_users'] === true;
            }))
            ->andReturn(['success' => true, 'restored' => ['files' => ['example.com'], 'databases' => ['db1']]]);

        $this->app->instance(AgentClientInterface::class, $mockAgent);

        $service = app(GranularRestoreService::class);
        $restore = $service->fullRestore($user, $snapshot);

        $this->assertInstanceOf(BackupRestore::class, $restore);
        $this->assertEquals('completed', $restore->status);
        $this->assertEquals($user->id, $restore->user_id);
        $this->assertDatabaseHas('backup_restores', [
            'user_id' => $user->id,
            'status' => 'completed',
            'restore_files' => true,
            'restore_databases' => true,
            'restore_mailboxes' => true,
            'restore_dns' => true,
        ]);
    }

    public function test_browse_snapshot_returns_categorized_contents(): void
    {
        $user = User::factory()->create(['username' => 'testuser']);

        $destination = BackupDestination::create([
            'user_id' => $user->id,
            'name' => 'Test SFTP',
            'type' => 'sftp',
            'config' => ['host' => '10.0.0.1', 'port' => 22, 'username' => 'backup'],
            'is_active' => true,
        ]);

        $snapshot = UserRemoteBackup::create([
            'user_id' => $user->id,
            'destination_id' => $destination->id,
            'backup_name' => '2026-03-25_170002',
            'backup_path' => '2026-03-25_170002/testuser',
            'backup_type' => 'incremental',
            'backup_date' => Carbon::parse('2026-03-25 17:00:02'),
        ]);

        $mockAgent = Mockery::mock(AgentClientInterface::class);
        $mockAgent->shouldReceive('send')->once()
            ->with('backup.list_contents', Mockery::type('array'))
            ->andReturn([
                'success' => true,
                'files' => [
                    'domains/example.com',
                    'domains/example.com/public_html',
                    'domains/example.com/public_html/index.php',
                    'domains/other.com',
                    'domains/other.com/public_html',
                    'mail/example.com',
                    'mail/example.com/info',
                    'mail/example.com/info/cur',
                    'mail/example.com/admin',
                    'mysql',
                    'mysql/db_wordpress.sql',
                    'mysql/db_shop.sql',
                    'mysql/users.sql',
                    'zones',
                    'zones/example.com.zone',
                    'zones/other.com.zone',
                    'ssl',
                    'ssl/example.com.crt',
                ],
            ]);

        $this->app->instance(AgentClientInterface::class, $mockAgent);

        $service = app(GranularRestoreService::class);
        $contents = $service->browseSnapshot($user, $snapshot);

        $this->assertArrayHasKey('domains', $contents);
        $this->assertArrayHasKey('databases', $contents);
        $this->assertArrayHasKey('mailboxes', $contents);
        $this->assertArrayHasKey('mysql_users', $contents);
        $this->assertArrayHasKey('dns_zones', $contents);
        $this->assertArrayHasKey('ssl_certificates', $contents);

        $this->assertEquals(['example.com', 'other.com'], $contents['domains']);
        $this->assertEquals(['db_wordpress.sql', 'db_shop.sql'], $contents['databases']);
        $this->assertEquals(['example.com/info', 'example.com/admin'], $contents['mailboxes']);
        $this->assertTrue($contents['mysql_users']);
        $this->assertEquals(['example.com.zone', 'other.com.zone'], $contents['dns_zones']);
        $this->assertEquals(['example.com.crt'], $contents['ssl_certificates']);
    }

    public function test_full_restore_rejects_path_traversal_in_snapshot(): void
    {
        $user = User::factory()->create(['username' => 'testuser']);

        $destination = BackupDestination::create([
            'user_id' => $user->id,
            'name' => 'Test',
            'type' => 'sftp',
            'config' => ['host' => '10.0.0.1'],
            'is_active' => true,
        ]);

        $snapshot = UserRemoteBackup::create([
            'user_id' => $user->id,
            'destination_id' => $destination->id,
            'backup_name' => '2026-03-25_170002',
            'backup_path' => '../../etc/passwd',
            'backup_type' => 'incremental',
            'backup_date' => Carbon::parse('2026-03-25 17:00:02'),
        ]);

        $mockAgent = Mockery::mock(AgentClientInterface::class);
        $mockAgent->shouldNotReceive('send');
        $this->app->instance(AgentClientInterface::class, $mockAgent);

        $service = app(GranularRestoreService::class);

        $this->expectException(RuntimeException::class);
        $service->fullRestore($user, $snapshot);
    }

    public function test_selective_restore_only_restores_selected_items(): void
    {
        $user = User::factory()->create(['username' => 'testuser']);

        $destination = BackupDestination::create([
            'user_id' => $user->id,
            'name' => 'Test SFTP',
            'type' => 'sftp',
            'config' => ['host' => '10.0.0.1'],
            'is_active' => true,
        ]);

        $snapshot = UserRemoteBackup::create([
            'user_id' => $user->id,
            'destination_id' => $destination->id,
            'backup_name' => '2026-03-25_170002',
            'backup_path' => '2026-03-25_170002/testuser',
            'backup_type' => 'incremental',
            'backup_date' => Carbon::parse('2026-03-25 17:00:02'),
        ]);

        $mockAgent = Mockery::mock(AgentClientInterface::class);
        $mockAgent->shouldReceive('send')->once()
            ->with('backup.restore', Mockery::on(function (array $params) {
                return $params['username'] === 'testuser'
                    && $params['restore_files'] === false
                    && $params['restore_databases'] === true
                    && $params['restore_mailboxes'] === true
                    && $params['restore_dns'] === false
                    && $params['restore_ssl'] === false
                    && $params['restore_mysql_users'] === false
                    && $params['selected_databases'] === ['db_wordpress.sql', 'db_shop.sql']
                    && $params['selected_mailboxes'] === ['example.com/info'];
            }))
            ->andReturn(['success' => true, 'restored' => ['databases' => ['db_wordpress', 'db_shop'], 'mailboxes' => ['info@example.com']]]);

        $this->app->instance(AgentClientInterface::class, $mockAgent);

        $service = app(GranularRestoreService::class);
        $restore = $service->selectiveRestore($user, $snapshot, [
            'databases' => ['db_wordpress.sql', 'db_shop.sql'],
            'mailboxes' => ['example.com/info'],
        ]);

        $this->assertEquals('completed', $restore->status);
        $this->assertTrue($restore->restore_databases);
        $this->assertTrue($restore->restore_mailboxes);
        $this->assertFalse($restore->restore_files);
        $this->assertFalse($restore->restore_dns);
        $this->assertEquals(['db_wordpress.sql', 'db_shop.sql'], $restore->selected_databases);
        $this->assertEquals(['example.com/info'], $restore->selected_mailboxes);
    }

    public function test_browse_domain_files_returns_directory_listing(): void
    {
        $user = User::factory()->create(['username' => 'testuser']);

        $destination = BackupDestination::create([
            'user_id' => $user->id,
            'name' => 'Test SFTP',
            'type' => 'sftp',
            'config' => ['host' => '10.0.0.1', 'port' => 22, 'username' => 'backup'],
            'is_active' => true,
        ]);

        $snapshot = UserRemoteBackup::create([
            'user_id' => $user->id,
            'destination_id' => $destination->id,
            'backup_name' => '2026-03-25_170002',
            'backup_path' => '2026-03-25_170002/testuser',
            'backup_type' => 'incremental',
            'backup_date' => Carbon::parse('2026-03-25 17:00:02'),
        ]);

        $mockAgent = Mockery::mock(AgentClientInterface::class);
        $mockAgent->shouldReceive('send')->once()
            ->with('backup.list_domain_files', Mockery::on(function (array $params) {
                return $params['path'] === 'domains/example.com/public_html'
                    && ! empty($params['snapshot_id']);
            }))
            ->andReturn([
                'success' => true,
                'items' => [
                    ['name' => 'wp-content', 'is_dir' => true, 'path' => 'domains/example.com/public_html/wp-content', 'size' => 0, 'permissions' => 'drwxr-xr-x', 'modified' => 1711360000],
                    ['name' => 'index.php', 'is_dir' => false, 'path' => 'domains/example.com/public_html/index.php', 'size' => 405, 'permissions' => '-rw-r--r--', 'modified' => 1711360000],
                    ['name' => 'wp-config.php', 'is_dir' => false, 'path' => 'domains/example.com/public_html/wp-config.php', 'size' => 3200, 'permissions' => '-rw-r--r--', 'modified' => 1711360000],
                ],
            ]);

        $this->app->instance(AgentClientInterface::class, $mockAgent);

        $service = app(GranularRestoreService::class);
        $listing = $service->browseDomainFiles($user, $snapshot, 'domains/example.com/public_html');

        $this->assertCount(3, $listing);
        $this->assertEquals('wp-content', $listing[0]['name']);
        $this->assertTrue($listing[0]['is_dir']);
        $this->assertEquals('index.php', $listing[1]['name']);
        $this->assertFalse($listing[1]['is_dir']);
    }

    public function test_selective_restore_with_specific_files(): void
    {
        $user = User::factory()->create(['username' => 'testuser']);

        $destination = BackupDestination::create([
            'user_id' => $user->id,
            'name' => 'Test',
            'type' => 'sftp',
            'config' => ['host' => '10.0.0.1'],
            'is_active' => true,
        ]);

        $snapshot = UserRemoteBackup::create([
            'user_id' => $user->id,
            'destination_id' => $destination->id,
            'backup_name' => '2026-03-25_170002',
            'backup_path' => '2026-03-25_170002/testuser',
            'backup_type' => 'incremental',
            'backup_date' => Carbon::parse('2026-03-25 17:00:02'),
        ]);

        $selectedFiles = [
            'domains/example.com/public_html/wp-content/uploads',
            'domains/example.com/public_html/wp-config.php',
        ];

        $mockAgent = Mockery::mock(AgentClientInterface::class);
        $mockAgent->shouldReceive('send')->once()
            ->with('backup.restore', Mockery::on(function (array $params) use ($selectedFiles) {
                return $params['restore_files'] === true
                    && $params['restore_databases'] === false
                    && $params['selected_files'] === $selectedFiles;
            }))
            ->andReturn(['success' => true, 'restored' => ['files' => $selectedFiles]]);

        $this->app->instance(AgentClientInterface::class, $mockAgent);

        $service = app(GranularRestoreService::class);
        $restore = $service->selectiveRestore($user, $snapshot, [
            'files' => $selectedFiles,
        ]);

        $this->assertEquals('completed', $restore->status);
        $this->assertTrue($restore->restore_files);
        $this->assertFalse($restore->restore_databases);
    }

    public function test_selective_restore_passes_conflict_resolution_to_agent(): void
    {
        $user = User::factory()->create(['username' => 'testuser']);

        $destination = BackupDestination::create([
            'user_id' => $user->id,
            'name' => 'Test',
            'type' => 'sftp',
            'config' => ['host' => '10.0.0.1'],
            'is_active' => true,
        ]);

        $snapshot = UserRemoteBackup::create([
            'user_id' => $user->id,
            'destination_id' => $destination->id,
            'backup_name' => '2026-03-25_170002',
            'backup_path' => '2026-03-25_170002/testuser',
            'backup_type' => 'incremental',
            'backup_date' => Carbon::parse('2026-03-25 17:00:02'),
        ]);

        $mockAgent = Mockery::mock(AgentClientInterface::class);
        $mockAgent->shouldReceive('send')->once()
            ->with('backup.restore', Mockery::on(function (array $params) {
                return $params['conflict_resolution']['databases'] === 'rename'
                    && $params['conflict_resolution']['mailboxes'] === 'merge'
                    && $params['conflict_resolution']['files'] === 'overwrite';
            }))
            ->andReturn(['success' => true, 'restored' => []]);

        $this->app->instance(AgentClientInterface::class, $mockAgent);

        $service = app(GranularRestoreService::class);
        $restore = $service->selectiveRestore($user, $snapshot, [
            'databases' => ['db_wordpress.sql'],
            'mailboxes' => ['example.com/info'],
            'domains' => ['example.com'],
        ], [
            'databases' => 'rename',
            'mailboxes' => 'merge',
            'files' => 'overwrite',
        ]);

        $this->assertEquals('completed', $restore->status);
    }

    public function test_safety_backup_created_before_restore_when_enabled(): void
    {
        $user = User::factory()->create(['username' => 'testuser']);

        $destination = BackupDestination::create([
            'user_id' => $user->id,
            'name' => 'Test',
            'type' => 'sftp',
            'config' => ['host' => '10.0.0.1'],
            'is_active' => true,
        ]);

        $snapshot = UserRemoteBackup::create([
            'user_id' => $user->id,
            'destination_id' => $destination->id,
            'backup_name' => '2026-03-25_170002',
            'backup_path' => '2026-03-25_170002/testuser',
            'backup_type' => 'incremental',
            'backup_date' => Carbon::parse('2026-03-25 17:00:02'),
        ]);

        $mockAgent = Mockery::mock(AgentClientInterface::class);

        // Safety backup call should come FIRST
        $mockAgent->shouldReceive('send')->once()
            ->with('backup.create', Mockery::on(fn (array $p) => $p['username'] === 'testuser'))
            ->andReturn(['success' => true, 'backup_path' => '/tmp/jabali_safety_testuser.tar.gz'])
            ->ordered();

        // Then the restore call
        $mockAgent->shouldReceive('send')->once()
            ->with('backup.restore', Mockery::type('array'))
            ->andReturn(['success' => true, 'restored' => []])
            ->ordered();

        $this->app->instance(AgentClientInterface::class, $mockAgent);

        $service = app(GranularRestoreService::class);
        $restore = $service->fullRestore($user, $snapshot, safetyBackup: true);

        $this->assertEquals('completed', $restore->status);
    }

    public function test_safety_backup_skipped_when_disabled(): void
    {
        $user = User::factory()->create(['username' => 'testuser']);

        $destination = BackupDestination::create([
            'user_id' => $user->id,
            'name' => 'Test',
            'type' => 'sftp',
            'config' => ['host' => '10.0.0.1'],
            'is_active' => true,
        ]);

        $snapshot = UserRemoteBackup::create([
            'user_id' => $user->id,
            'destination_id' => $destination->id,
            'backup_name' => '2026-03-25_170002',
            'backup_path' => '2026-03-25_170002/testuser',
            'backup_type' => 'incremental',
            'backup_date' => Carbon::parse('2026-03-25 17:00:02'),
        ]);

        $mockAgent = Mockery::mock(AgentClientInterface::class);

        // Should NOT receive backup.create
        $mockAgent->shouldNotReceive('send')->with('backup.create', Mockery::any());

        // Only the restore call
        $mockAgent->shouldReceive('send')->once()
            ->with('backup.restore', Mockery::type('array'))
            ->andReturn(['success' => true, 'restored' => []]);

        $this->app->instance(AgentClientInterface::class, $mockAgent);

        $service = app(GranularRestoreService::class);
        $restore = $service->fullRestore($user, $snapshot, safetyBackup: false);

        $this->assertEquals('completed', $restore->status);
    }

    public function test_partial_failure_completes_with_summary(): void
    {
        $user = User::factory()->create(['username' => 'testuser']);

        $destination = BackupDestination::create([
            'user_id' => $user->id,
            'name' => 'Test',
            'type' => 'sftp',
            'config' => ['host' => '10.0.0.1'],
            'is_active' => true,
        ]);

        $snapshot = UserRemoteBackup::create([
            'user_id' => $user->id,
            'destination_id' => $destination->id,
            'backup_name' => '2026-03-25_170002',
            'backup_path' => '2026-03-25_170002/testuser',
            'backup_type' => 'incremental',
            'backup_date' => Carbon::parse('2026-03-25 17:00:02'),
        ]);

        $mockAgent = Mockery::mock(AgentClientInterface::class);
        $mockAgent->shouldReceive('send')->once()
            ->with('backup.restore', Mockery::type('array'))
            ->andReturn([
                'success' => true,
                'restored' => [
                    'databases' => ['db_wordpress'],
                    'mailboxes' => ['info@example.com'],
                ],
                'failed' => [
                    ['type' => 'database', 'name' => 'db_shop.sql', 'error' => 'Corrupted dump file'],
                ],
            ]);

        $this->app->instance(AgentClientInterface::class, $mockAgent);

        $service = app(GranularRestoreService::class);
        $restore = $service->selectiveRestore($user, $snapshot, [
            'databases' => ['db_wordpress.sql', 'db_shop.sql'],
            'mailboxes' => ['example.com/info'],
        ]);

        // Restore should still be completed (not failed) since some items succeeded
        $this->assertEquals('completed', $restore->status);

        $result = $restore->result;
        $this->assertArrayHasKey('restored', $result);
        $this->assertArrayHasKey('failed', $result);
        $this->assertEquals(['db_wordpress'], $result['restored']['databases']);
        $this->assertEquals('db_shop.sql', $result['failed'][0]['name']);
        $this->assertEquals('Corrupted dump file', $result['failed'][0]['error']);
    }

    public function test_non_owner_cannot_restore_another_users_snapshot(): void
    {
        $owner = User::factory()->create(['username' => 'owner']);
        $attacker = User::factory()->create(['username' => 'attacker']);

        $destination = BackupDestination::create([
            'user_id' => $owner->id,
            'name' => 'Test',
            'type' => 'sftp',
            'config' => ['host' => '10.0.0.1'],
            'is_active' => true,
        ]);

        $snapshot = UserRemoteBackup::create([
            'user_id' => $owner->id,
            'destination_id' => $destination->id,
            'backup_name' => '2026-03-25_170002',
            'backup_path' => '2026-03-25_170002/owner',
            'backup_type' => 'incremental',
            'backup_date' => Carbon::parse('2026-03-25 17:00:02'),
        ]);

        $mockAgent = Mockery::mock(AgentClientInterface::class);
        $mockAgent->shouldNotReceive('send');
        $this->app->instance(AgentClientInterface::class, $mockAgent);

        $service = app(GranularRestoreService::class);

        $this->expectException(RuntimeException::class);
        $service->fullRestore($attacker, $snapshot);
    }

    public function test_admin_can_restore_for_any_user(): void
    {
        $admin = User::factory()->admin()->create(['username' => 'admin']);
        $targetUser = User::factory()->create(['username' => 'targetuser']);

        $destination = BackupDestination::create([
            'user_id' => $targetUser->id,
            'name' => 'Test',
            'type' => 'sftp',
            'config' => ['host' => '10.0.0.1'],
            'is_active' => true,
        ]);

        $snapshot = UserRemoteBackup::create([
            'user_id' => $targetUser->id,
            'destination_id' => $destination->id,
            'backup_name' => '2026-03-25_170002',
            'backup_path' => '2026-03-25_170002/targetuser',
            'backup_type' => 'incremental',
            'backup_date' => Carbon::parse('2026-03-25 17:00:02'),
        ]);

        $mockAgent = Mockery::mock(AgentClientInterface::class);
        $mockAgent->shouldReceive('send')->once()
            ->with('backup.restore', Mockery::on(fn (array $p) => $p['username'] === 'targetuser'))
            ->andReturn(['success' => true, 'restored' => []]);

        $this->app->instance(AgentClientInterface::class, $mockAgent);

        $service = app(GranularRestoreService::class);
        $restore = $service->fullRestore($targetUser, $snapshot, performedBy: $admin);

        $this->assertEquals('completed', $restore->status);
        $this->assertEquals($targetUser->id, $restore->user_id);
    }

    public function test_restore_records_performed_by_for_audit(): void
    {
        $admin = User::factory()->admin()->create(['username' => 'admin']);
        $targetUser = User::factory()->create(['username' => 'targetuser']);

        $destination = BackupDestination::create([
            'user_id' => $targetUser->id,
            'name' => 'Test',
            'type' => 'sftp',
            'config' => ['host' => '10.0.0.1'],
            'is_active' => true,
        ]);

        $snapshot = UserRemoteBackup::create([
            'user_id' => $targetUser->id,
            'destination_id' => $destination->id,
            'backup_name' => '2026-03-25_170002',
            'backup_path' => '2026-03-25_170002/targetuser',
            'backup_type' => 'incremental',
            'backup_date' => Carbon::parse('2026-03-25 17:00:02'),
        ]);

        $mockAgent = Mockery::mock(AgentClientInterface::class);
        $mockAgent->shouldReceive('send')->once()
            ->with('backup.restore', Mockery::type('array'))
            ->andReturn(['success' => true, 'restored' => []]);

        $this->app->instance(AgentClientInterface::class, $mockAgent);

        $service = app(GranularRestoreService::class);
        $restore = $service->fullRestore($targetUser, $snapshot, performedBy: $admin);

        $this->assertEquals($targetUser->id, $restore->user_id);
        $this->assertDatabaseHas('backup_restores', [
            'user_id' => $targetUser->id,
            'performed_by' => $admin->id,
            'snapshot_id' => $snapshot->id,
        ]);
    }
}
