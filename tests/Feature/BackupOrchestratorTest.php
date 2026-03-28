<?php

declare(strict_types=1);

namespace Tests\Feature;

use App\Models\Backup;
use App\Services\Agent\AgentClient;
use App\Services\Backup\BackupOrchestrator;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Mockery;
use Tests\TestCase;

class BackupOrchestratorTest extends TestCase
{
    use RefreshDatabase;

    public function test_execute_runs_full_backup_and_marks_completed(): void
    {
        $backup = Backup::create([
            'name' => 'Test Backup',
            'filename' => 'test-backup.tar.gz',
            'type' => 'server',
            'status' => 'pending',
            'local_path' => '/tmp/test-backup.tar.gz',
        ]);

        $agent = Mockery::mock(AgentClient::class);
        $agent->shouldReceive('send')
            ->with('backup.create_server', Mockery::type('array'))
            ->once()
            ->andReturn([
                'success' => true,
                'snapshot_id' => 'abc12345',
                'size' => 1024000,
                'users' => ['admin'],
                'user_count' => 1,
            ]);

        $orchestrator = new BackupOrchestrator($agent);
        $orchestrator->execute($backup);

        $backup->refresh();
        $this->assertEquals('completed', $backup->status);
        $this->assertEquals(1024000, $backup->size_bytes);
        $this->assertNotNull($backup->completed_at);
    }

    public function test_execute_marks_failed_on_agent_error(): void
    {
        $backup = Backup::create([
            'name' => 'Failing Backup',
            'filename' => 'fail-backup.tar.gz',
            'type' => 'server',
            'status' => 'pending',
            'local_path' => '/tmp/fail-backup.tar.gz',
        ]);

        $agent = Mockery::mock(AgentClient::class);
        $agent->shouldReceive('send')
            ->with('backup.create_server', Mockery::type('array'))
            ->once()
            ->andThrow(new \Exception('Disk full'));

        $orchestrator = new BackupOrchestrator($agent);
        $orchestrator->execute($backup);

        $backup->refresh();
        $this->assertEquals('failed', $backup->status);
        $this->assertEquals('Disk full', $backup->error_message);
    }
}
