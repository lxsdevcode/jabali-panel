<?php

declare(strict_types=1);

namespace Tests\Feature\Cli;

use App\Console\Commands\Cli\SystemDiskCommand;
use App\Console\Commands\Cli\SystemHostnameCommand;
use App\Console\Commands\Cli\SystemInfoCommand;
use App\Console\Commands\Cli\SystemMemoryCommand;
use App\Console\Commands\Cli\SystemStatusCommand;
use Tests\TestCase;

class SystemCommandTest extends TestCase
{
    protected function setUp(): void
    {
        parent::setUp();
        $kernel = $this->app[\Illuminate\Contracts\Console\Kernel::class];
        $kernel->registerCommand(new SystemInfoCommand);
        $kernel->registerCommand(new SystemStatusCommand);
        $kernel->registerCommand(new SystemHostnameCommand);
        $kernel->registerCommand(new SystemDiskCommand);
        $kernel->registerCommand(new SystemMemoryCommand);
    }

    public function test_system_info_shows_hostname(): void
    {
        $hostname = gethostname() ?: 'unknown';

        $this->artisan('jabali:system:info')
            ->assertExitCode(0)
            ->expectsOutputToContain($hostname);
    }

    public function test_system_disk_shows_usage(): void
    {
        $this->artisan('jabali:system:disk')
            ->assertExitCode(0)
            ->expectsOutputToContain('/');
    }

    public function test_system_hostname_without_args_shows_current(): void
    {
        $hostname = gethostname() ?: 'unknown';

        $this->artisan('jabali:system:hostname')
            ->assertExitCode(0)
            ->expectsOutputToContain($hostname);
    }
}
