<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use Illuminate\Console\Command;
use Illuminate\Support\Facades\Artisan;

class HelpCommand extends Command
{
    protected $signature = 'jabali:help';

    protected $description = 'Show available Jabali CLI commands';

    public function handle(): int
    {
        $versionFile = base_path('VERSION');
        $version = '0.0.0';
        if (file_exists($versionFile)) {
            $content = file_get_contents($versionFile);
            if (preg_match('/VERSION=(.+)/', $content, $m)) {
                $version = trim($m[1]);
            }
        }

        $this->line('');
        $this->line("<options=bold>Jabali CLI</> v{$version}");
        $this->line(str_repeat('─', 40));
        $this->line('');
        $this->line('<fg=yellow>Usage:</>  jabali <noun> <verb> [options]');
        $this->line('');

        // Discover all jabali:* commands and group by noun
        $commands = Artisan::all();
        $groups = [];
        $standalone = [];

        foreach ($commands as $name => $command) {
            if (! str_starts_with($name, 'jabali:')) {
                continue;
            }
            if ($name === 'jabali:help') {
                continue;
            }

            $parts = explode(':', $name, 3);
            if (count($parts) === 3) {
                $groups[$parts[1]][$parts[2]] = $command->getDescription();
            } else {
                // Two-part commands (jabali:smoke-test → "jabali smoke-test")
                $standalone[$parts[1]] = $command->getDescription();
            }
        }

        ksort($groups);

        foreach ($groups as $noun => $verbs) {
            $this->line("  <fg=green>{$noun}</>");
            ksort($verbs);
            foreach ($verbs as $verb => $description) {
                $padded = str_pad($verb, 20);
                $this->line("    {$padded}<fg=gray>{$description}</>");
            }
        }

        // Standalone commands
        $this->line('');
        $this->line('  <fg=green>update</>');
        $this->line('    '.str_pad('', 20).'<fg=gray>Update Jabali Panel to latest version</>');

        $standalone = array_filter($standalone, fn ($k) => $k !== 'upgrade', ARRAY_FILTER_USE_KEY);
        if (! empty($standalone)) {
            $this->line('');
            $this->line('<fg=yellow>Utilities:</>');
            ksort($standalone);
            foreach ($standalone as $cmd => $description) {
                $padded = str_pad($cmd, 22);
                $this->line("  {$padded}<fg=gray>{$description}</>");
            }
        }

        $this->line('');
        $this->line('<fg=yellow>Global options:</>');
        $this->line('  --json              Output as JSON');
        $this->line('  --quiet             Suppress output (exit code only)');
        $this->line('  --yes               Skip confirmation prompts');
        $this->line('  --help              Show this help');
        $this->line('  --version           Show version');
        $this->line('');

        return Command::SUCCESS;
    }
}
