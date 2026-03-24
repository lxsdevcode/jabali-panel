<?php

declare(strict_types=1);

namespace Tests\Unit;

use App\Models\BackupDestination;
use App\Services\Agent\AgentClient;
use App\Services\Backup\BackupOrchestrator;
use Mockery;
use Tests\TestCase;

class BackupOrchestratorTest extends TestCase
{
    private AgentClient $mockAgent;

    private BackupOrchestrator $orchestrator;

    protected function setUp(): void
    {
        parent::setUp();
        $this->mockAgent = Mockery::mock(AgentClient::class);
        $this->orchestrator = new BackupOrchestrator($this->mockAgent);
    }

    public function test_build_destination_config_merges_type(): void
    {
        $destination = new BackupDestination;
        $destination->type = 'sftp';
        $destination->config = ['host' => '10.0.0.1', 'port' => 22, 'username' => 'backup'];

        $config = $this->orchestrator->buildDestinationConfig($destination);

        $this->assertSame('sftp', $config['type']);
        $this->assertSame('10.0.0.1', $config['host']);
        $this->assertSame(22, $config['port']);
        $this->assertSame('backup', $config['username']);
    }

    public function test_build_destination_config_handles_null_config(): void
    {
        $destination = new BackupDestination;
        $destination->type = 's3';
        $destination->config = null;

        $config = $this->orchestrator->buildDestinationConfig($destination);

        $this->assertSame(['type' => 's3'], $config);
    }

    public function test_validate_path_rejects_traversal(): void
    {
        $this->expectException(\Exception::class);

        $method = new \ReflectionMethod(BackupOrchestrator::class, 'validatePath');
        $method->invoke($this->orchestrator, '/home/../etc/passwd');
    }

    public function test_validate_path_rejects_invalid_prefix(): void
    {
        $this->expectException(\Exception::class);

        $method = new \ReflectionMethod(BackupOrchestrator::class, 'validatePath');
        $method->invoke($this->orchestrator, '/etc/shadow');
    }

    public function test_validate_path_accepts_valid_backup_path(): void
    {
        $method = new \ReflectionMethod(BackupOrchestrator::class, 'validatePath');

        $method->invoke($this->orchestrator, '/var/backups/jabali/2026-01-01');
        $method->invoke($this->orchestrator, '/home/user1/backups/backup.tar.gz');

        $this->assertTrue(true);
    }
}
