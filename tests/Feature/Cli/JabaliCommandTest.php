<?php

declare(strict_types=1);

namespace Tests\Feature\Cli;

use App\Console\Cli\JabaliCommand;
use App\Services\Agent\AgentClientInterface;
use Illuminate\Console\Command;
use Tests\TestCase;

/**
 * Concrete test command that extends the base class.
 */
class StubCommand extends JabaliCommand
{
    protected $signature = 'jabali:stub:list {--json} {--yes}';

    protected $description = 'Stub command for testing';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $result = $this->callAgent('user.list');

        if ($result === null) {
            return Command::FAILURE;
        }

        $data = $result->get('users', []);

        if ($this->option('json')) {
            $this->formatter()->json($data);
        } else {
            $this->formatter()->table(
                ['Name', 'Email'],
                array_map(fn ($u) => [$u['name'], $u['email']], $data),
            );
        }

        return Command::SUCCESS;
    }
}

class StubDeleteCommand extends JabaliCommand
{
    protected $signature = 'jabali:stub:delete {--json} {--yes}';

    protected $description = 'Stub delete command for testing';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        if (! $this->confirmAction('Are you sure you want to delete?')) {
            return Command::SUCCESS;
        }

        $result = $this->callAgent('user.delete', ['id' => 1]);

        return $result ? Command::SUCCESS : Command::FAILURE;
    }
}

class JabaliCommandTest extends TestCase
{
    protected function setUp(): void
    {
        parent::setUp();
        $this->app[\Illuminate\Contracts\Console\Kernel::class]->registerCommand(new StubCommand);
    }

    public function test_json_flag_produces_json_output(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn([
            'success' => true,
            'users' => [
                ['name' => 'Alice', 'email' => 'alice@example.com'],
            ],
        ]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:stub:list', ['--json' => true])
            ->assertExitCode(0);
    }

    public function test_quiet_flag_suppresses_output(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn([
            'success' => true,
            'users' => [
                ['name' => 'Alice', 'email' => 'alice@example.com'],
            ],
        ]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:stub:list', ['--quiet' => true])
            ->doesntExpectOutput()
            ->assertExitCode(0);
    }

    public function test_yes_flag_skips_confirmation(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn([
            'success' => true,
            'users' => [],
        ]);
        $this->app->instance(AgentClientInterface::class, $agent);

        // Register a command that requires confirmation
        $this->app[\Illuminate\Contracts\Console\Kernel::class]->registerCommand(new StubDeleteCommand);

        // With --yes, should not prompt and should succeed
        $this->artisan('jabali:stub:delete', ['--yes' => true])
            ->assertExitCode(0);
    }

    public function test_failed_agent_call_returns_exit_code_1(): void
    {
        $agent = $this->createMock(AgentClientInterface::class);
        $agent->method('send')->willReturn([
            'success' => false,
            'error' => 'User not found',
        ]);
        $this->app->instance(AgentClientInterface::class, $agent);

        $this->artisan('jabali:stub:list')
            ->assertExitCode(1);
    }
}
