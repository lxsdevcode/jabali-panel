<?php

declare(strict_types=1);

namespace Tests\Unit;

use App\Services\Agent\AgentClient;
use App\Services\FileBrowser\BackupSnapshotFileOperations;
use Exception;
use Mockery;
use Mockery\MockInterface;
use PHPUnit\Framework\TestCase;

class BackupSnapshotFileOperationsTest extends TestCase
{
    private AgentClient&MockInterface $agent;

    private BackupSnapshotFileOperations $ops;

    protected function setUp(): void
    {
        parent::setUp();

        $this->agent = Mockery::mock(AgentClient::class);
        $this->ops = new BackupSnapshotFileOperations(
            agent: $this->agent,
            snapshotId: 'snap-123',
            username: 'testuser',
            repo: '/backups/restic',
        );
    }

    protected function tearDown(): void
    {
        Mockery::close();
        parent::tearDown();
    }

    // --- Path mapping ---

    public function test_list_empty_path_uses_home(): void
    {
        $this->agent
            ->shouldReceive('send')
            ->once()
            ->withArgs(function (string $action, array $params): bool {
                return $action === 'backup.list_domain_files'
                    && $params['path'] === '/home';
            })
            ->andReturn(['items' => []]);

        $result = $this->ops->list('');

        $this->assertSame(['items' => []], $result);
    }

    public function test_list_relative_path_prepends_home(): void
    {
        $this->agent
            ->shouldReceive('send')
            ->once()
            ->withArgs(function (string $action, array $params): bool {
                return $params['path'] === '/home/shuki';
            })
            ->andReturn(['items' => []]);

        $result = $this->ops->list('shuki');

        $this->assertSame(['items' => []], $result);
    }

    public function test_list_nested_path_prepends_home(): void
    {
        $this->agent
            ->shouldReceive('send')
            ->once()
            ->withArgs(function (string $action, array $params): bool {
                return $params['path'] === '/home/shuki/domains';
            })
            ->andReturn(['items' => []]);

        $result = $this->ops->list('shuki/domains');

        $this->assertSame(['items' => []], $result);
    }

    // --- Path traversal rejection ---

    public function test_list_rejects_dot_dot_traversal(): void
    {
        $this->agent->shouldNotReceive('send');

        $result = $this->ops->list('../etc/passwd');

        $this->assertSame(['items' => []], $result);
    }

    public function test_list_rejects_mid_path_traversal(): void
    {
        $this->agent->shouldNotReceive('send');

        $result = $this->ops->list('shuki/../../etc');

        $this->assertSame(['items' => []], $result);
    }

    public function test_list_backslash_traversal_is_not_caught_by_inline_regex(): void
    {
        // BackupSnapshotFileOperations uses a simple regex that only matches
        // forward-slash traversal. Backslash traversal passes through to the agent.
        // PathSanitizer::clean() handles backslash conversion separately.
        $this->agent
            ->shouldReceive('send')
            ->once()
            ->andReturn(['items' => []]);

        $result = $this->ops->list('shuki\\..\\..\\etc');

        $this->assertSame(['items' => []], $result);
    }

    // --- Current dir name filtered ---

    public function test_list_filters_current_directory_from_results(): void
    {
        $this->agent
            ->shouldReceive('send')
            ->andReturn([
                'items' => [
                    ['name' => 'home', 'is_dir' => true, 'size' => null, 'modified' => 1000, 'permissions' => '0755'],
                    ['name' => 'user1', 'is_dir' => true, 'size' => null, 'modified' => 1000, 'permissions' => '0755'],
                    ['name' => 'user2', 'is_dir' => true, 'size' => null, 'modified' => 1000, 'permissions' => '0755'],
                ],
            ]);

        $result = $this->ops->list('');

        // "home" is basename of /home, so it should be filtered out
        $names = array_column($result['items'], 'name');
        $this->assertNotContains('home', $names);
        $this->assertContains('user1', $names);
        $this->assertContains('user2', $names);
    }

    public function test_list_filters_dot_and_dotdot_entries(): void
    {
        $this->agent
            ->shouldReceive('send')
            ->andReturn([
                'items' => [
                    ['name' => '.', 'is_dir' => true],
                    ['name' => '..', 'is_dir' => true],
                    ['name' => 'real-file.txt', 'is_dir' => false, 'size' => 100, 'modified' => 1000, 'permissions' => '0644'],
                ],
            ]);

        $result = $this->ops->list('shuki');

        $names = array_column($result['items'], 'name');
        $this->assertNotContains('.', $names);
        $this->assertNotContains('..', $names);
        $this->assertContains('real-file.txt', $names);
    }

    public function test_list_filters_empty_name_entries(): void
    {
        $this->agent
            ->shouldReceive('send')
            ->andReturn([
                'items' => [
                    ['name' => '', 'is_dir' => false],
                    ['name' => 'valid.txt', 'is_dir' => false, 'size' => 50, 'modified' => 1000, 'permissions' => '0644'],
                ],
            ]);

        $result = $this->ops->list('shuki');

        $this->assertCount(1, $result['items']);
        $this->assertSame('valid.txt', $result['items'][0]['name']);
    }

    // --- Hidden files ---

    public function test_list_filters_hidden_files_when_show_hidden_false(): void
    {
        $this->agent
            ->shouldReceive('send')
            ->andReturn([
                'items' => [
                    ['name' => '.bashrc', 'is_dir' => false, 'size' => 200, 'modified' => 1000, 'permissions' => '0644'],
                    ['name' => '.config', 'is_dir' => true, 'size' => null, 'modified' => 1000, 'permissions' => '0755'],
                    ['name' => 'readme.txt', 'is_dir' => false, 'size' => 100, 'modified' => 1000, 'permissions' => '0644'],
                ],
            ]);

        $result = $this->ops->list('shuki', showHidden: false);

        $names = array_column($result['items'], 'name');
        $this->assertNotContains('.bashrc', $names);
        $this->assertNotContains('.config', $names);
        $this->assertContains('readme.txt', $names);
    }

    public function test_list_shows_hidden_files_when_show_hidden_true(): void
    {
        $this->agent
            ->shouldReceive('send')
            ->andReturn([
                'items' => [
                    ['name' => '.bashrc', 'is_dir' => false, 'size' => 200, 'modified' => 1000, 'permissions' => '0644'],
                    ['name' => 'readme.txt', 'is_dir' => false, 'size' => 100, 'modified' => 1000, 'permissions' => '0644'],
                ],
            ]);

        $result = $this->ops->list('shuki', showHidden: true);

        $names = array_column($result['items'], 'name');
        $this->assertContains('.bashrc', $names);
        $this->assertContains('readme.txt', $names);
    }

    // --- Sorting ---

    public function test_list_sorts_directories_first_then_alphabetical(): void
    {
        $this->agent
            ->shouldReceive('send')
            ->andReturn([
                'items' => [
                    ['name' => 'zebra.txt', 'is_dir' => false, 'size' => 10, 'modified' => 1000, 'permissions' => '0644'],
                    ['name' => 'alpha', 'is_dir' => true, 'size' => null, 'modified' => 1000, 'permissions' => '0755'],
                    ['name' => 'apple.txt', 'is_dir' => false, 'size' => 20, 'modified' => 1000, 'permissions' => '0644'],
                    ['name' => 'beta', 'is_dir' => true, 'size' => null, 'modified' => 1000, 'permissions' => '0755'],
                ],
            ]);

        $result = $this->ops->list('shuki', showHidden: true);

        $names = array_column($result['items'], 'name');
        $this->assertSame(['alpha', 'beta', 'apple.txt', 'zebra.txt'], $names);
    }

    public function test_list_sort_is_case_insensitive(): void
    {
        $this->agent
            ->shouldReceive('send')
            ->andReturn([
                'items' => [
                    ['name' => 'Zebra.txt', 'is_dir' => false, 'size' => 10, 'modified' => 1000, 'permissions' => '0644'],
                    ['name' => 'apple.txt', 'is_dir' => false, 'size' => 20, 'modified' => 1000, 'permissions' => '0644'],
                    ['name' => 'Banana.txt', 'is_dir' => false, 'size' => 30, 'modified' => 1000, 'permissions' => '0644'],
                ],
            ]);

        $result = $this->ops->list('shuki', showHidden: true);

        $names = array_column($result['items'], 'name');
        $this->assertSame(['apple.txt', 'Banana.txt', 'Zebra.txt'], $names);
    }

    // --- Agent exception ---

    public function test_list_returns_empty_items_on_agent_exception(): void
    {
        $this->agent
            ->shouldReceive('send')
            ->andThrow(new Exception('Connection refused'));

        $result = $this->ops->list('shuki');

        $this->assertSame(['items' => []], $result);
    }

    // --- Item structure ---

    public function test_list_item_has_correct_structure(): void
    {
        $this->agent
            ->shouldReceive('send')
            ->andReturn([
                'items' => [
                    ['name' => 'test.txt', 'is_dir' => false, 'size' => 1024, 'modified' => 1700000000, 'permissions' => '0644'],
                ],
            ]);

        $result = $this->ops->list('shuki', showHidden: true);

        $this->assertCount(1, $result['items']);
        $item = $result['items'][0];
        $this->assertSame('test.txt', $item['name']);
        $this->assertSame('shuki/test.txt', $item['path']);
        $this->assertFalse($item['is_dir']);
        $this->assertSame(1024, $item['size']);
        $this->assertSame(1700000000, $item['modified']);
        $this->assertSame('0644', $item['permissions']);
    }

    public function test_list_directory_item_has_null_size(): void
    {
        $this->agent
            ->shouldReceive('send')
            ->andReturn([
                'items' => [
                    ['name' => 'subdir', 'is_dir' => true, 'size' => null, 'modified' => 1700000000, 'permissions' => '0755'],
                ],
            ]);

        $result = $this->ops->list('shuki', showHidden: true);

        $this->assertNull($result['items'][0]['size']);
        $this->assertTrue($result['items'][0]['is_dir']);
    }

    public function test_list_empty_path_item_path_has_no_prefix(): void
    {
        $this->agent
            ->shouldReceive('send')
            ->andReturn([
                'items' => [
                    ['name' => 'user1', 'is_dir' => true, 'size' => null, 'modified' => 1000, 'permissions' => '0755'],
                ],
            ]);

        $result = $this->ops->list('');

        // When path is empty, item path should just be the name (no leading slash)
        $this->assertSame('user1', $result['items'][0]['path']);
    }

    // --- Write operations throw ---

    public function test_read_throws_read_only(): void
    {
        $this->expectException(\RuntimeException::class);
        $this->expectExceptionMessage('read-only');

        $this->ops->read('any/path');
    }

    public function test_write_throws_read_only(): void
    {
        $this->expectException(\RuntimeException::class);
        $this->expectExceptionMessage('read-only');

        $this->ops->write('any/path', 'content');
    }

    public function test_delete_throws_read_only(): void
    {
        $this->expectException(\RuntimeException::class);
        $this->expectExceptionMessage('read-only');

        $this->ops->delete('any/path');
    }

    public function test_mkdir_throws_read_only(): void
    {
        $this->expectException(\RuntimeException::class);
        $this->expectExceptionMessage('read-only');

        $this->ops->mkdir('any/path');
    }

    // --- Agent params ---

    public function test_list_passes_snapshot_id_and_repo(): void
    {
        $this->agent
            ->shouldReceive('send')
            ->once()
            ->withArgs(function (string $action, array $params): bool {
                return $params['snapshot_id'] === 'snap-123'
                    && $params['username'] === 'testuser'
                    && $params['repo'] === '/backups/restic';
            })
            ->andReturn(['items' => []]);

        $result = $this->ops->list('');

        $this->assertSame(['items' => []], $result);
    }
}
