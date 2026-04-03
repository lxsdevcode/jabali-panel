<?php

declare(strict_types=1);

namespace Tests\Feature;

use App\Livewire\Admin\PhpExtensions;
use App\Models\User;
use App\Services\Agent\AgentClient;
use Livewire\Livewire;
use Tests\TestCase;

class PhpExtensionTest extends TestCase
{
    /** @var array<int, array{name: string, enabled: bool, installed: bool}> */
    private static array $fixtures = [
        ['name' => 'gd', 'enabled' => true, 'installed' => true],
        ['name' => 'mbstring', 'enabled' => false, 'installed' => true],
        ['name' => 'imagick', 'enabled' => false, 'installed' => false],
    ];

    private function makeAgent(array &$calls): AgentClient
    {
        $fixtures = self::$fixtures;
        $agent = $this->createMock(AgentClient::class);
        $agent->method('send')->willReturnCallback(
            function (string $action, array $params) use (&$calls, $fixtures): array {
                $calls[] = compact('action', 'params');

                return $action === 'php.list_extensions'
                    ? ['success' => true, 'extensions' => $fixtures]
                    : ['success' => true];
            }
        );

        return $agent;
    }

    private function admin(): User
    {
        return User::factory()->admin()->make();
    }

    public function test_table_renders_extensions_for_selected_version(): void
    {
        $calls = [];
        app()->instance(AgentClient::class, $this->makeAgent($calls));

        Livewire::actingAs($this->admin())
            ->test(PhpExtensions::class, ['version' => '8.3'])
            ->assertSee('gd')
            ->assertSee('mbstring')
            ->assertSee('imagick');

        $this->assertCount(1, $calls);
        $this->assertSame('php.list_extensions', $calls[0]['action']);
        $this->assertSame('8.3', $calls[0]['params']['version']);
    }

    public function test_enable_extension_dispatches_correct_rpc(): void
    {
        $calls = [];
        app()->instance(AgentClient::class, $this->makeAgent($calls));

        Livewire::actingAs($this->admin())
            ->test(PhpExtensions::class, ['version' => '8.3'])
            ->call('enableExtension', 'mbstring');

        $actionCalls = array_filter($calls, fn ($c) => $c['action'] === 'php.enable_extension');
        $this->assertNotEmpty($actionCalls);

        $call = array_values($actionCalls)[0];
        $this->assertSame('8.3', $call['params']['version']);
        $this->assertSame('mbstring', $call['params']['extension']);
    }

    public function test_disable_extension_dispatches_correct_rpc(): void
    {
        $calls = [];
        app()->instance(AgentClient::class, $this->makeAgent($calls));

        Livewire::actingAs($this->admin())
            ->test(PhpExtensions::class, ['version' => '8.3'])
            ->call('disableExtension', 'gd');

        $actionCalls = array_filter($calls, fn ($c) => $c['action'] === 'php.disable_extension');
        $this->assertNotEmpty($actionCalls);

        $call = array_values($actionCalls)[0];
        $this->assertSame('8.3', $call['params']['version']);
        $this->assertSame('gd', $call['params']['extension']);
    }

    public function test_install_extension_dispatches_correct_rpc(): void
    {
        $calls = [];
        app()->instance(AgentClient::class, $this->makeAgent($calls));

        Livewire::actingAs($this->admin())
            ->test(PhpExtensions::class, ['version' => '8.3'])
            ->call('installExtension', 'imagick');

        $actionCalls = array_filter($calls, fn ($c) => $c['action'] === 'php.install_extension');
        $this->assertNotEmpty($actionCalls);

        $call = array_values($actionCalls)[0];
        $this->assertSame('8.3', $call['params']['version']);
        $this->assertSame('imagick', $call['params']['extension']);
    }

    public function test_remove_extension_dispatches_correct_rpc(): void
    {
        $calls = [];
        app()->instance(AgentClient::class, $this->makeAgent($calls));

        Livewire::actingAs($this->admin())
            ->test(PhpExtensions::class, ['version' => '8.3'])
            ->call('removeExtension', 'mbstring');

        $actionCalls = array_filter($calls, fn ($c) => $c['action'] === 'php.remove_extension');
        $this->assertNotEmpty($actionCalls);

        $call = array_values($actionCalls)[0];
        $this->assertSame('8.3', $call['params']['version']);
        $this->assertSame('mbstring', $call['params']['extension']);
    }

    public function test_invalid_version_format_is_rejected_before_agent_call(): void
    {
        $calls = [];
        app()->instance(AgentClient::class, $this->makeAgent($calls));

        Livewire::actingAs($this->admin())
            ->test(PhpExtensions::class, ['version' => 'invalid-version'])
            ->assertSet('version', '')
            ->assertSet('extensions', []);

        $this->assertEmpty($calls);
    }

    public function test_invalid_extension_name_is_rejected_before_agent_call(): void
    {
        $calls = [];
        app()->instance(AgentClient::class, $this->makeAgent($calls));

        Livewire::actingAs($this->admin())
            ->test(PhpExtensions::class, ['version' => '8.3'])
            ->call('enableExtension', '../../etc/passwd');

        $actionCalls = array_filter($calls, fn ($c) => $c['action'] === 'php.enable_extension');
        $this->assertEmpty($actionCalls);
    }
}
